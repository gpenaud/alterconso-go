package service

import (
	"errors"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type VolunteerService struct {
	db *gorm.DB
}

func NewVolunteerService(db *gorm.DB) *VolunteerService {
	return &VolunteerService{db: db}
}

// GetForDistrib retourne les bénévoles inscrits pour un MultiDistrib.
func (s *VolunteerService) GetForDistrib(multiDistribID uint) ([]model.Volunteer, error) {
	var volunteers []model.Volunteer
	err := s.db.Preload("User").
		Where("multi_distrib_id = ?", multiDistribID).
		Find(&volunteers).Error
	return volunteers, err
}

// Register inscrit un utilisateur comme bénévole pour une distribution.
func (s *VolunteerService) Register(userID, multiDistribID uint, role *string) (*model.Volunteer, error) {
	// Vérifier que la distribution existe
	var md model.MultiDistrib
	if err := s.db.First(&md, multiDistribID).Error; err != nil {
		return nil, errors.New("distribution not found")
	}
	if md.Validated {
		return nil, errors.New("distribution is already validated")
	}

	// Vérifier qu'il n'est pas déjà inscrit pour ce rôle
	var count int64
	q := s.db.Model(&model.Volunteer{}).
		Where("user_id = ? AND multi_distrib_id = ?", userID, multiDistribID)
	if role != nil {
		q = q.Where("role = ?", *role)
	}
	q.Count(&count)
	if count > 0 {
		return nil, errors.New("already registered as volunteer for this distribution")
	}

	v := &model.Volunteer{
		UserID:         userID,
		MultiDistribID: multiDistribID,
		Role:           role,
	}
	return v, s.db.Create(v).Error
}

// Unregister désinscrit un bénévole.
func (s *VolunteerService) Unregister(volunteerID, userID uint) error {
	result := s.db.Where("id = ? AND user_id = ?", volunteerID, userID).
		Delete(&model.Volunteer{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("volunteer entry not found")
	}
	return nil
}
