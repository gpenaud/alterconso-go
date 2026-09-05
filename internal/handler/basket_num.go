package handler

import (
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// basketNumbers retourne le numéro de panier de chaque adhérent pour une
// distribution.
//
// La lecture porte sur la distribution entière, jamais sur un catalogue seul :
// c'est ce qui fait qu'un adhérent porte le même numéro sur la liste de chaque
// producteur. Un rang calculé sur les seules commandes affichées — ce que
// faisaient ces écrans — lui en donnait un par producteur.
//
// Un adhérent absent de la table ressort à 0, et les gabarits taisent alors le
// numéro plutôt que d'afficher un « N°0 » qui ne désigne rien.
func basketNumbers(db *gorm.DB, multiDistribID uint) map[uint]int {
	nums := make(map[uint]int)
	var baskets []model.Basket
	if err := db.Where("multi_distrib_id = ?", multiDistribID).Find(&baskets).Error; err != nil {
		return nums
	}
	for _, b := range baskets {
		nums[b.UserID] = b.Num
	}
	return nums
}
