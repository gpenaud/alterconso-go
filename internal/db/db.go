package db

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Silent
	if cfg.Debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		// Désactive la création des contraintes FK lors du AutoMigrate.
		// Évite les erreurs de dépendances circulaires (ex: Group ↔ Place).
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// BackfillVerifiedUsers : marque comme "email vérifié" tous les comptes
// existants créés avant l'introduction du flux d'activation par email.
// Critère : compte avec mot de passe défini ET EmailVerifiedAt nil.
func BackfillVerifiedUsers(db *gorm.DB) error {
	return db.Exec(
		"UPDATE users SET email_verified_at = NOW() WHERE email_verified_at IS NULL AND pass <> ''",
	).Error
}

// EnsureSuperAdmin garantit l'existence du compte administrateur global défini
// dans la config. Idempotent : à exécuter à chaque démarrage.
//   - crée le compte s'il n'existe pas (avec Rights bit 0 activé)
//   - active Rights bit 0 (admin site-wide) sur un compte existant
//   - met à jour le mot de passe si fourni dans la config
//   - marque l'email comme vérifié
//
// Si cfg.SuperAdmin.Email est vide, ne fait rien.
func EnsureSuperAdmin(db *gorm.DB, cfg *config.Config) error {
	sa := cfg.SuperAdmin
	if strings.TrimSpace(sa.Email) == "" {
		return nil
	}
	email := strings.ToLower(strings.TrimSpace(sa.Email))

	var u model.User
	err := db.Where("email = ?", email).First(&u).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return err
	}
	if isNew {
		u = model.User{Email: email}
	}
	if sa.FirstName != "" {
		u.FirstName = sa.FirstName
	}
	if sa.LastName != "" {
		u.LastName = sa.LastName
	}
	u.Rights |= 1
	if sa.Password != "" {
		u.SetPassword(sa.Password, cfg.Key)
	}
	if u.EmailVerifiedAt == nil {
		now := time.Now()
		u.EmailVerifiedAt = &now
	}
	return db.Save(&u).Error
}

// EnsureLegalRepAdmins garantit que chaque représentant légal de groupe
// possède le droit GroupAdmin. À exécuter après Migrate au démarrage.
func EnsureLegalRepAdmins(db *gorm.DB) error {
	var groups []model.Group
	if err := db.Where("legal_representative_id IS NOT NULL").Find(&groups).Error; err != nil {
		return err
	}
	for _, g := range groups {
		if g.LegalRepresentativeID == nil {
			continue
		}
		var ug model.UserGroup
		if err := db.Where("user_id = ? AND group_id = ?", *g.LegalRepresentativeID, g.ID).First(&ug).Error; err != nil {
			continue
		}
		rights := ug.GetRights()
		hasAdmin := false
		for _, r := range rights {
			if r.Right == model.RightGroupAdmin {
				hasAdmin = true
				break
			}
		}
		if hasAdmin {
			continue
		}
		rights = append(rights, model.UserRight{Right: model.RightGroupAdmin})
		raw, err := json.Marshal(rights)
		if err != nil {
			continue
		}
		ug.Rights = string(raw)
		db.Save(&ug)
	}
	return nil
}

// Migrate crée ou met à jour les tables à partir des modèles Go.
// DisableForeignKeyConstraintWhenMigrating étant activé, l'ordre n'a pas d'importance.
// GORM ne supprime jamais de colonnes existantes — safe en production.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Vendor{},
		&model.Group{},
		&model.Place{},
		&model.UserGroup{},
		&model.Catalog{},
		&model.Product{},
		&model.MultiDistrib{},
		&model.Distribution{},
		&model.Subscription{},
		&model.Basket{},
		&model.UserOrder{},
		&model.Operation{},
		&model.Message{},
		&model.Membership{},
		&model.WaitingList{},
		&model.Volunteer{},
		&model.VolunteerRole{},
		&model.GroupDoc{},
		&model.NotificationSent{},
		&model.PasswordResetToken{},
		&model.EmailVerifyToken{},
		&model.TxpCategory{},
		&model.TxpSubCategory{},
	)
}

// SeedTxpCategories insère les 10 catégories taxonomiques par défaut si la
// table est vide. Idempotent : ne fait rien si des catégories existent déjà.
// Chaque catégorie reçoit une sous-catégorie "Tous" pour permettre au shop
// legacy de fonctionner même quand les produits n'ont pas de subcategorie
// assignée individuellement.
func SeedTxpCategories(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.TxpCategory{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	seeds := []struct {
		Name         string
		Image        string
		DisplayOrder int
	}{
		{"Fruits et légumes", "fruits-legumes", 1},
		{"Viande et charcuterie", "viande-charcuterie", 2},
		{"Produits de la mer", "produits-mer", 3},
		{"Boulangerie / pâtisserie", "boulangerie-patisserie", 4},
		{"Crémerie", "cremerie", 5},
		{"Desserts et plats préparés", "desserts-plats-prepares", 6},
		{"Épicerie", "epicerie", 7},
		{"Boissons", "boissons", 8},
		{"Hygiène", "hygiene", 9},
		{"Autres", "autres", 99},
	}
	for _, s := range seeds {
		cat := model.TxpCategory{Name: s.Name, Image: s.Image, DisplayOrder: s.DisplayOrder}
		if err := db.Create(&cat).Error; err != nil {
			return err
		}
		sub := model.TxpSubCategory{Name: "Tous", CategoryID: cat.ID}
		if err := db.Create(&sub).Error; err != nil {
			return err
		}
	}
	return nil
}
