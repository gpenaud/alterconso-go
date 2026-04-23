package model

import "time"

// Place : lieu de distribution.
type Place struct {
	ID      uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name    string `gorm:"size:64;not null" json:"name"`
	Address *string `gorm:"size:128" json:"address,omitempty"`
	ZipCode *string `gorm:"size:32"  json:"zipCode,omitempty"`
	City    *string `gorm:"size:64"  json:"city,omitempty"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`

	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`
}

func (p *Place) TableName() string { return "places" }

// Basket : panier en mode boutique.
type Basket struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`

	UserID uint `json:"-"`
	User   User `gorm:"foreignKey:UserID" json:"-"`

	MultiDistribID uint         `json:"-"`
	MultiDistrib   MultiDistrib `gorm:"foreignKey:MultiDistribID" json:"-"`

	Orders []UserOrder `gorm:"foreignKey:BasketID" json:"-"`
}

func (b *Basket) TableName() string { return "baskets" }

// Subscription : souscription à un catalogue AMAP (commandes fixes).
type Subscription struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`

	UserID uint `json:"-"`
	User   User `gorm:"foreignKey:UserID" json:"-"`

	CatalogID uint    `json:"-"`
	Catalog   Catalog `gorm:"foreignKey:CatalogID" json:"-"`

	StartDate time.Time  `json:"startDate"`
	EndDate   *time.Time `json:"endDate,omitempty"`

	// Quantité de chaque produit commandé (JSON sérialisé)
	Quantities string `gorm:"type:text" json:"-"`
}

func (s *Subscription) TableName() string { return "subscriptions" }

// Message : message interne dans un groupe.
type Message struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`

	SenderID uint  `json:"-"`
	Sender   User  `gorm:"foreignKey:SenderID" json:"sender"`
	GroupID  uint  `json:"-"`
	Group    Group `gorm:"foreignKey:GroupID" json:"-"`

	Subject string `gorm:"size:128;not null" json:"subject"`
	Body    string `gorm:"type:text;not null" json:"body"`
}

func (m *Message) TableName() string { return "messages" }

// Membership : adhésion annuelle d'un membre à un groupe.
type Membership struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`

	UserID  uint  `json:"-"`
	User    User  `gorm:"foreignKey:UserID" json:"-"`
	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	Year int     `json:"year"`
	Fee  float64 `json:"fee"`
}

func (m *Membership) TableName() string { return "memberships" }

// WaitingList : liste d'attente pour un catalogue.
type WaitingList struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`

	UserID    uint    `json:"-"`
	User      User    `gorm:"foreignKey:UserID" json:"-"`
	CatalogID uint    `json:"-"`
	Catalog   Catalog `gorm:"foreignKey:CatalogID" json:"-"`

	Message *string `gorm:"type:text" json:"message,omitempty"`
}

func (w *WaitingList) TableName() string { return "waiting_lists" }

// Volunteer : bénévole inscrit pour une permanence.
type Volunteer struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`

	UserID         uint         `json:"-"`
	User           User         `gorm:"foreignKey:UserID" json:"user"`
	MultiDistribID uint         `json:"-"`
	MultiDistrib   MultiDistrib `gorm:"foreignKey:MultiDistribID" json:"-"`

	Role *string `gorm:"size:64" json:"role,omitempty"`
}

func (v *Volunteer) TableName() string { return "volunteers" }

// VolunteerRole : rôle de bénévole défini pour un groupe, rattaché à un catalogue.
type VolunteerRole struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"-"`

	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	CatalogID *uint    `json:"-"`
	Catalog   *Catalog `gorm:"foreignKey:CatalogID" json:"catalog,omitempty"`

	Name string `gorm:"size:128" json:"name"`
}

func (vr *VolunteerRole) TableName() string { return "volunteer_roles" }

// MultiDistribRole : rôle sélectionné pour une distribution donnée.
type MultiDistribRole struct {
	MultiDistribID   uint `gorm:"primaryKey"`
	VolunteerRoleID  uint `gorm:"primaryKey"`
}

func (MultiDistribRole) TableName() string { return "multi_distrib_roles" }
