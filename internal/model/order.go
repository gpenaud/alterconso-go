package model

import "time"

// OrderFlag : options d'une commande.
type OrderFlag uint

const (
	OrderFlagInvertSharedOrder OrderFlag = 1 << iota // inverser le tour pour les commandes partagées
)

// UserOrder : une commande passée par un utilisateur.
type UserOrder struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"date"`

	// Commanditaire principal
	UserID uint `json:"-"`
	User   User `gorm:"foreignKey:UserID" json:"user"`

	// Commanditaire secondaire (commande partagée / alternée)
	User2ID *uint `json:"-"`
	User2   *User `gorm:"foreignKey:User2ID" json:"user2,omitempty"`

	// Produit commandé
	ProductID uint    `json:"-"`
	Product   Product `gorm:"foreignKey:ProductID" json:"product"`

	Quantity float64 `gorm:"not null;default:1" json:"quantity"`

	// Prix et frais au moment de la commande (snapshot)
	ProductPrice float64 `gorm:"default:0" json:"productPrice"`
	FeesRate     float64 `gorm:"default:0" json:"feesRate"`

	Paid bool `gorm:"default:false" json:"paid"`

	// Distribution associée (commandes variables)
	DistributionID *uint         `json:"-"`
	Distribution   *Distribution `gorm:"foreignKey:DistributionID" json:"-"`

	// Panier (shop mode)
	BasketID *uint   `json:"-"`
	Basket   *Basket `gorm:"foreignKey:BasketID" json:"-"`

	// Souscription (commandes fixes AMAP)
	SubscriptionID *uint         `json:"-"`
	Subscription   *Subscription `gorm:"foreignKey:SubscriptionID" json:"-"`

	Flags uint `gorm:"default:0" json:"-"`
}

func (o *UserOrder) TableName() string { return "user_orders" }

func (o *UserOrder) HasFlag(f OrderFlag) bool {
	return OrderFlag(o.Flags)&f != 0
}

// CanModify retourne true si la commande est encore modifiable.
func (o *UserOrder) CanModify() bool {
	if o.Paid {
		return false
	}
	if o.Product.Catalog.Type == CatalogTypeVarOrder {
		if o.Distribution == nil {
			return false
		}
		return o.Distribution.CanOrderNow()
	}
	return o.Product.Catalog.UsersCanOrder()
}

// TotalPrice retourne le prix total TTC de la commande avec frais.
func (o *UserOrder) TotalPrice() float64 {
	base := o.Quantity * o.ProductPrice
	return base * (1 + o.FeesRate/100)
}
