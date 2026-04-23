package model

import "time"

// LegalStatus : statut juridique d'un producteur.
type LegalStatus string

const (
	LegalStatusSoletrader  LegalStatus = "Soletrader"
	LegalStatusOrganization LegalStatus = "Organization"
	LegalStatusBusiness    LegalStatus = "Business"
)

// Vendor : producteur / fournisseur.
type Vendor struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	Name        string      `gorm:"size:64;not null"  json:"name"`
	Email       string      `gorm:"size:64;not null"  json:"email"`
	Phone       *string     `gorm:"size:19"           json:"phone,omitempty"`
	Address1    *string     `gorm:"size:64"           json:"address1,omitempty"`
	ZipCode     *string     `gorm:"size:32"           json:"zipCode,omitempty"`
	City        *string     `gorm:"size:64"           json:"city,omitempty"`
	Description *string     `gorm:"type:text"         json:"description,omitempty"`
	LegalStatus *LegalStatus `gorm:"size:32"          json:"legalStatus,omitempty"`
	Organic     bool        `gorm:"default:false"     json:"organic"`

	// Image (chemin vers le fichier uploadé)
	ImagePath *string `gorm:"size:255" json:"image,omitempty"`

	Catalogs []Catalog `gorm:"foreignKey:VendorID" json:"-"`
}

func (v *Vendor) TableName() string { return "vendors" }
