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

	// Le groupe qui a saisi cette fiche. Un producteur n'appartient a
	// personne — plusieurs groupes commandent chez le meme — et c'est un
	// catalogue qui le rattache reellement. Mais un producteur tout juste
	// cree n'en a pas encore : sans cette trace, il n'apparaitrait sur
	// l'ecran d'aucun groupe, et celui qui vient de le saisir ne pourrait
	// plus le retrouver pour lui ouvrir un catalogue.
	//
	// Nullable, et pour de bon : les fiches anterieures n'ont pas de groupe
	// createur, et leur en attribuer un au hasard d'une migration donnerait
	// pour un fait ce qui ne serait qu'une supposition.
	GroupID *uint `gorm:"index" json:"-"`

	Catalogs []Catalog `gorm:"foreignKey:VendorID" json:"-"`
}

func (v *Vendor) TableName() string { return "vendors" }
