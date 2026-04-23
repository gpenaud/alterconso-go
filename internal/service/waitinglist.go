package service

import (
	"errors"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type WaitingListService struct {
	db *gorm.DB
}

func NewWaitingListService(db *gorm.DB) *WaitingListService {
	return &WaitingListService{db: db}
}

// GetForCatalog retourne la liste d'attente d'un catalogue.
func (s *WaitingListService) GetForCatalog(catalogID uint) ([]model.WaitingList, error) {
	var list []model.WaitingList
	err := s.db.Preload("User").
		Where("catalog_id = ?", catalogID).
		Order("created_at ASC").Find(&list).Error
	return list, err
}

// Join inscrit un utilisateur sur la liste d'attente.
func (s *WaitingListService) Join(userID, catalogID uint, message *string) (*model.WaitingList, error) {
	// Vérifier que l'utilisateur n'est pas déjà inscrit
	var count int64
	s.db.Model(&model.WaitingList{}).
		Where("user_id = ? AND catalog_id = ?", userID, catalogID).Count(&count)
	if count > 0 {
		return nil, errors.New("already on waiting list")
	}

	entry := &model.WaitingList{
		UserID:    userID,
		CatalogID: catalogID,
		Message:   message,
	}
	return entry, s.db.Create(entry).Error
}

// Leave supprime un utilisateur de la liste d'attente.
func (s *WaitingListService) Leave(userID, catalogID uint) error {
	result := s.db.Where("user_id = ? AND catalog_id = ?", userID, catalogID).
		Delete(&model.WaitingList{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("not on waiting list")
	}
	return nil
}

// Position retourne la position d'un utilisateur dans la liste (1-based), 0 si absent.
func (s *WaitingListService) Position(userID, catalogID uint) int {
	var list []model.WaitingList
	s.db.Select("user_id").
		Where("catalog_id = ?", catalogID).
		Order("created_at ASC").Find(&list)
	for i, e := range list {
		if e.UserID == userID {
			return i + 1
		}
	}
	return 0
}
