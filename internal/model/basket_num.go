package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureBasket retourne le panier de l'adhérent pour cette distribution, en le
// créant — numéro compris — s'il n'existe pas encore.
//
// Le numéro identifie l'adhérent pendant toute la distribution : il doit donc
// être le même chez tous les producteurs présents ce jour-là. C'est pourquoi il
// est porté par le panier, dont la portée est le MultiDistrib, et non recalculé
// à l'affichage — un rang calculé sur les commandes d'un seul catalogue redonne
// à la même personne un numéro par producteur.
func EnsureBasket(db *gorm.DB, userID, multiDistribID uint) (*Basket, error) {
	// Trois tentatives : une commande passée au même instant pour le même
	// adhérent peut créer le panier entre notre lecture et notre insertion. On
	// relit alors le panier qu'elle a créé plutôt que d'en faire un second.
	for essai := 0; essai < 3; essai++ {
		var b Basket
		err := db.Where("user_id = ? AND multi_distrib_id = ?", userID, multiDistribID).
			First(&b).Error
		if err == nil {
			return &b, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		b = Basket{UserID: userID, MultiDistribID: multiDistribID}
		err = db.Transaction(func(tx *gorm.DB) error {
			var max int
			// Le verrou court jusqu'à l'insertion : deux paniers créés en même
			// temps sur la même distribution se suivent au lieu de lire tous
			// deux le même maximum et de réclamer le même numéro.
			row := tx.Model(&Basket{}).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("multi_distrib_id = ?", multiDistribID).
				Select("COALESCE(MAX(num), 0)").
				Row()
			if err := row.Scan(&max); err != nil {
				return err
			}
			b.Num = max + 1
			return tx.Create(&b).Error
		})
		if err == nil {
			return &b, nil
		}
		if !isDuplicateKey(err) {
			return nil, err
		}
	}
	return nil, errors.New("panier : numéro introuvable après trois tentatives")
}

// isDuplicateKey reconnaît la violation d'unicité, seule erreur d'insertion
// qu'il vaille la peine de retenter.
func isDuplicateKey(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// Le pilote MySQL ne renvoie pas toujours l'erreur typée de GORM selon la
	// façon dont la requête a été construite : on retombe sur le code 1062.
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "1062")
}

// BeforeCreate rattache la commande au panier de son adhérent, en le créant au
// besoin.
//
// Le rattachement est fait ici plutôt que chez les appelants parce que c'est
// précisément ce qu'ils oubliaient : sur les cinq chemins qui créent une
// commande, un seul posait le panier. Les quatre autres — saisie par un
// responsable, ajout d'un produit, API de compatibilité — laissaient
// l'adhérent sans panier, donc sans numéro et sans rattachement de ses
// paiements. Un chemin ajouté demain hériterait du même oubli.
func (o *UserOrder) BeforeCreate(tx *gorm.DB) error {
	if o.BasketID != nil || o.DistributionID == nil {
		return nil
	}

	var distrib Distribution
	if err := tx.Select("multi_distrib_id").First(&distrib, *o.DistributionID).Error; err != nil {
		// Une distribution introuvable est le problème de l'appelant, pas
		// celui du panier : refuser la commande ici masquerait la vraie cause.
		return nil
	}

	basket, err := EnsureBasket(tx, o.UserID, distrib.MultiDistribID)
	if err != nil {
		return err
	}
	o.BasketID = &basket.ID
	return nil
}
