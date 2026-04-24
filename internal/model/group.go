package model

import "time"

// GroupFlag représente les options binaires d'un groupe.
type GroupFlag uint

const (
	GroupFlagShopMode             GroupFlag = 1 << iota // mode boutique
	GroupFlagHasPayments                                // gestion des paiements
	GroupFlagComputeMargin                              // marge au lieu de pourcentage
	GroupFlagCagetteNetwork                             // inscrit dans l'annuaire
	GroupFlagCustomizedCategories                       // catégories personnalisées
	GroupFlagHidePhone                                  // masquer téléphone du responsable
	GroupFlagPhoneRequired                              // téléphone obligatoire
	GroupFlagAddressRequired                            // adresse obligatoire
)

// RegOption : mode d'inscription des nouveaux membres.
type RegOption string

const (
	RegOptionClosed      RegOption = "Closed"
	RegOptionWaitingList RegOption = "WaitingList"
	RegOptionOpen        RegOption = "Open"
	RegOptionFull        RegOption = "Full"
)

// GroupType : type de groupe coopératif.
type GroupType string

const (
	GroupTypeAmap          GroupType = "Amap"
	GroupTypeGroupedOrders GroupType = "GroupedOrders"
	GroupTypeProducerDrive GroupType = "ProducerDrive"
	GroupTypeFarmShop      GroupType = "FarmShop"
)

// Group correspond à la table `groups`.
type Group struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	Name string `gorm:"size:64;not null" json:"name"`

	// Textes descriptifs
	TxtIntro  *string `gorm:"type:text" json:"txtIntro,omitempty"`
	TxtHome   *string `gorm:"type:text" json:"txtHome,omitempty"`
	TxtDistrib *string `gorm:"type:text" json:"txtDistrib,omitempty"`
	ExtURL    *string `gorm:"size:64"   json:"extUrl,omitempty"`

	// Contact et représentant légal
	ContactID           *uint  `json:"-"`
	Contact             *User  `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	LegalRepresentativeID *uint `json:"-"`
	LegalRepresentative *User  `gorm:"foreignKey:LegalRepresentativeID" json:"legalRepresentative,omitempty"`

	// Adhésions
	MembershipRenewalDate *time.Time `json:"membershipRenewalDate,omitempty"`
	MembershipFee         *int       `json:"membershipFee,omitempty"`
	HasMembership         bool       `gorm:"default:false" json:"hasMembership"`

	// Type et options
	GroupType GroupType `gorm:"size:32" json:"groupType"`
	RegOption RegOption `gorm:"size:32;default:'Open'" json:"regOption"`
	Flags     uint      `gorm:"default:0" json:"-"`

	// Devise
	Currency     string `gorm:"size:12;default:'€'"  json:"currency"`
	CurrencyCode string `gorm:"size:3;default:'EUR'" json:"currencyCode"`

	// Paiements
	AllowedPaymentsType            *string `gorm:"size:255" json:"-"`
	CheckOrder                     *string `gorm:"size:64"  json:"-"`
	IBAN                           *string `gorm:"size:40"  json:"-"`
	AllowMoneyPotWithNegativeBalance *bool  `json:"-"`

	// Bénévolat
	VolunteersMailDaysBeforeDutyPeriod      int    `gorm:"default:4"  json:"-"`
	VolunteersMailContent                   string `gorm:"type:text"  json:"-"`
	VacantVolunteerRolesMailDaysBeforeDutyPeriod int `gorm:"default:7" json:"-"`
	DaysBeforeDutyPeriodsOpen               int    `gorm:"default:60" json:"-"`
	AlertMailContent                        string `gorm:"type:text"  json:"-"`

	// Lieu principal (cache)
	MainPlaceID *uint  `json:"-"`
	MainPlace   *Place `gorm:"foreignKey:MainPlaceID" json:"mainPlace,omitempty"`

	// Logo (FK vers File)
	LogoID *uint `gorm:"column:logoId" json:"-"`
	Logo   *File `gorm:"foreignKey:LogoID" json:"-"`

	// Taux de TVA (4 paires nom/taux)
	VatName1 string  `gorm:"size:32" json:"-"`
	VatRate1 float64 `gorm:"default:0" json:"-"`
	VatName2 string  `gorm:"size:32" json:"-"`
	VatRate2 float64 `gorm:"default:0" json:"-"`
	VatName3 string  `gorm:"size:32" json:"-"`
	VatRate3 float64 `gorm:"default:0" json:"-"`
	VatName4 string  `gorm:"size:32" json:"-"`
	VatRate4 float64 `gorm:"default:0" json:"-"`

	// Relations
	Members  []UserGroup    `gorm:"foreignKey:GroupID" json:"-"`
	Places   []Place        `gorm:"foreignKey:GroupID" json:"-"`
	Catalogs []Catalog      `gorm:"foreignKey:GroupID" json:"-"`
}

func (g *Group) TableName() string { return "groups" }

// HasFlag vérifie si un flag de groupe est activé.
func (g *Group) HasFlag(f GroupFlag) bool {
	return GroupFlag(g.Flags)&f != 0
}

// SetFlag active un flag de groupe.
func (g *Group) SetFlag(f GroupFlag) {
	g.Flags = uint(GroupFlag(g.Flags) | f)
}

func (g *Group) HasShopMode() bool   { return g.HasFlag(GroupFlagShopMode) }
func (g *Group) HasPayments() bool   { return g.HasFlag(GroupFlagHasPayments) }
func (g *Group) CanExposePhone() bool { return !g.HasFlag(GroupFlagHidePhone) }
