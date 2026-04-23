package model

import "time"

// Operation : transaction financière (dette de commande ou paiement).
// On remplace l'ancien model misc.go Operation par cette version complète.
type Operation struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"date"`

	UserID  uint  `json:"-"`
	User    User  `gorm:"foreignKey:UserID" json:"-"`
	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	Amount      float64 `json:"amount"`
	Type        string  `gorm:"size:32" json:"type"` // VOrder, COrder, Payment, Membership
	Description *string `gorm:"size:255" json:"description,omitempty"`
	Pending     bool    `gorm:"default:true" json:"pending"`

	// Paiement
	PaymentType *string `gorm:"size:32" json:"paymentType,omitempty"`
	RemoteOpID  *string `gorm:"size:128" json:"remoteOpId,omitempty"`

	// Lien vers une opération parente (ex: paiement lié à une commande)
	RelatedOpID *uint       `json:"-"`
	RelatedOp   *Operation  `gorm:"foreignKey:RelatedOpID" json:"-"`

	// Panier associé (VOrder)
	BasketID *uint   `json:"-"`
	Basket   *Basket `gorm:"foreignKey:BasketID" json:"-"`
}

func (o *Operation) TableName() string { return "operations" }
