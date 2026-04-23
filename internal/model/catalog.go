package model

import "time"

// CatalogType : type de contrat / catalogue.
type CatalogType int

const (
	CatalogTypeVarOrder   CatalogType = 0 // commandes variables (paniers libres)
	CatalogTypeConstOrder CatalogType = 1 // commandes fixes (AMAP classique)
)

// CatalogFlag : options du catalogue.
type CatalogFlag uint

const (
	CatalogFlagUsersCanOrder     CatalogFlag = 1 << iota // les membres peuvent commander en ligne
	CatalogFlagStockManagement                           // gestion des stocks
	CatalogFlagHasFloatQt                                // quantités décimales autorisées
	CatalogFlagHasPercentageFees                         // frais en pourcentage
)

// Catalog : contrat entre un groupe et un producteur.
type Catalog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	Name      string      `gorm:"size:64;not null" json:"name"`
	Type      CatalogType `gorm:"default:0"        json:"type"`
	Flags     uint        `gorm:"default:0"        json:"-"`

	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`

	// Frais additionnels
	PercentageFees *float64 `json:"percentageFees,omitempty"`
	PercentageName *string  `gorm:"size:64" json:"percentageName,omitempty"`

	// Clé étrangère groupe
	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	// Clé étrangère producteur
	VendorID uint   `json:"-"`
	Vendor   Vendor `gorm:"foreignKey:VendorID" json:"vendor"`

	// Responsable du contrat
	ContactID *uint `json:"-"`
	Contact   *User `gorm:"foreignKey:ContactID" json:"contact,omitempty"`

	Products      []Product      `gorm:"foreignKey:CatalogID" json:"-"`
	Distributions []Distribution `gorm:"foreignKey:CatalogID" json:"-"`
}

func (c *Catalog) TableName() string { return "catalogs" }

func (c *Catalog) HasFlag(f CatalogFlag) bool {
	return CatalogFlag(c.Flags)&f != 0
}

func (c *Catalog) SetFlag(f CatalogFlag) {
	c.Flags = uint(CatalogFlag(c.Flags) | f)
}

func (c *Catalog) IsActive() bool {
	now := time.Now()
	if c.StartDate != nil && now.Before(*c.StartDate) {
		return false
	}
	if c.EndDate != nil && now.After(*c.EndDate) {
		return false
	}
	return true
}

func (c *Catalog) UsersCanOrder() bool {
	return c.HasFlag(CatalogFlagUsersCanOrder)
}
