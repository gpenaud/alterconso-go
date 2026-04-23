package model

import "time"

// MultiDistrib regroupe plusieurs distributions (d'un même jour / même lieu).
type MultiDistrib struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"-"`

	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	PlaceID uint  `json:"-"`
	Place   Place `gorm:"foreignKey:PlaceID" json:"place"`

	DistribStartDate time.Time  `json:"distribStartDate"`
	DistribEndDate   time.Time  `json:"distribEndDate"`
	OrderStartDate   *time.Time `json:"orderStartDate,omitempty"`
	OrderEndDate     *time.Time `json:"orderEndDate,omitempty"`

	Validated bool `gorm:"default:false" json:"validated"`

	Distributions []Distribution `gorm:"foreignKey:MultiDistribID" json:"-"`
}

func (m *MultiDistrib) TableName() string { return "multi_distribs" }

func (m *MultiDistrib) IsValidated() bool { return m.Validated }

// Distribution : une livraison pour un catalogue donné lors d'un MultiDistrib.
type Distribution struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"-"`

	CatalogID uint    `json:"-"`
	Catalog   Catalog `gorm:"foreignKey:CatalogID" json:"catalog"`

	MultiDistribID uint         `json:"-"`
	MultiDistrib   MultiDistrib `gorm:"foreignKey:MultiDistribID" json:"-"`

	// Dates spécifiques à cette distribution (surchargent celles du MultiDistrib si non nil)
	Date           *time.Time `json:"date,omitempty"`
	End            *time.Time `json:"end,omitempty"`
	OrderStartDate *time.Time `json:"orderStartDate,omitempty"`
	OrderEndDate   *time.Time `json:"orderEndDate,omitempty"`

	// Lieu (déprécié, utiliser MultiDistrib.Place)
	PlaceID *uint  `json:"-"`
	Place   *Place `gorm:"foreignKey:PlaceID" json:"-"`

	Orders []UserOrder `gorm:"foreignKey:DistributionID" json:"-"`
}

func (d *Distribution) TableName() string { return "distributions" }

// EffectiveDate retourne la date réelle (Distribution ou MultiDistrib).
func (d *Distribution) EffectiveDate() time.Time {
	if d.Date != nil {
		return *d.Date
	}
	return d.MultiDistrib.DistribStartDate
}

// CanOrderNow retourne true si les commandes sont ouvertes pour cette distribution.
func (d *Distribution) CanOrderNow() bool {
	orderEnd := d.OrderEndDate
	orderStart := d.OrderStartDate
	if orderEnd == nil {
		orderEnd = d.MultiDistrib.OrderEndDate
		orderStart = d.MultiDistrib.OrderStartDate
	}
	if orderEnd == nil {
		return d.Catalog.UsersCanOrder()
	}
	now := time.Now()
	return d.Catalog.UsersCanOrder() &&
		now.After(*orderStart) &&
		now.Before(*orderEnd)
}
