package service

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type SubscriptionService struct {
	db *gorm.DB
}

func NewSubscriptionService(db *gorm.DB) *SubscriptionService {
	return &SubscriptionService{db: db}
}

// QuantityMap mappe productID → quantité commandée.
type QuantityMap map[uint]float64

func (q QuantityMap) Marshal() (string, error) {
	b, err := json.Marshal(q)
	return string(b), err
}

func parseQuantities(raw string) QuantityMap {
	m := make(QuantityMap)
	if raw == "" {
		return m
	}
	json.Unmarshal([]byte(raw), &m) //nolint
	return m
}

// GetForUser retourne les abonnements actifs d'un utilisateur pour un catalogue.
func (s *SubscriptionService) GetForUser(userID, catalogID uint) ([]model.Subscription, error) {
	var subs []model.Subscription
	err := s.db.Where("user_id = ? AND catalog_id = ?", userID, catalogID).
		Order("start_date DESC").Find(&subs).Error
	return subs, err
}

// Subscribe crée ou met à jour un abonnement.
// quantities : map productID → quantité.
func (s *SubscriptionService) Subscribe(userID, catalogID uint, quantities QuantityMap, startDate time.Time) (*model.Subscription, error) {
	// Vérifier que le catalogue existe et est de type ConstOrder
	var catalog model.Catalog
	if err := s.db.First(&catalog, catalogID).Error; err != nil {
		return nil, errors.New("catalog not found")
	}
	if catalog.Type != model.CatalogTypeConstOrder {
		return nil, errors.New("subscriptions are only for AMAP (const order) catalogs")
	}

	raw, err := quantities.Marshal()
	if err != nil {
		return nil, err
	}

	// Chercher un abonnement actif existant
	var existing model.Subscription
	now := time.Now()
	err = s.db.Where("user_id = ? AND catalog_id = ? AND start_date <= ? AND (end_date IS NULL OR end_date >= ?)",
		userID, catalogID, now, now).First(&existing).Error

	if err == nil {
		// Mettre à jour les quantités
		existing.Quantities = raw
		return &existing, s.db.Save(&existing).Error
	}

	sub := &model.Subscription{
		UserID:    userID,
		CatalogID: catalogID,
		StartDate: startDate,
		Quantities: raw,
	}
	return sub, s.db.Create(sub).Error
}

// Unsubscribe clôture un abonnement en fixant EndDate = maintenant.
func (s *SubscriptionService) Unsubscribe(subID, userID uint) error {
	var sub model.Subscription
	if err := s.db.Where("id = ? AND user_id = ?", subID, userID).First(&sub).Error; err != nil {
		return errors.New("subscription not found")
	}
	now := time.Now()
	sub.EndDate = &now
	return s.db.Save(&sub).Error
}

// GetQuantities retourne la map de quantités d'un abonnement.
func (s *SubscriptionService) GetQuantities(sub *model.Subscription) QuantityMap {
	return parseQuantities(sub.Quantities)
}
