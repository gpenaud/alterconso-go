package model

// UnitType : unité de mesure d'un produit.
type UnitType string

const (
	UnitTypePiece      UnitType = "Piece"
	UnitTypeKilogram   UnitType = "Kilogram"
	UnitTypeGram       UnitType = "Gram"
	UnitTypeLitre      UnitType = "Litre"
	UnitTypeCentilitre UnitType = "Centilitre"
	UnitTypeMillilitre UnitType = "Millilitre"
)

// Product : produit dans un catalogue.
type Product struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`

	Name        string   `gorm:"size:255;not null" json:"name"`
	Ref         *string  `gorm:"size:64"           json:"ref,omitempty"`
	Description *string  `gorm:"type:text"         json:"description,omitempty"`
	Qt          *float64 `json:"qt,omitempty"`
	Price       float64  `gorm:"not null"          json:"price"`
	VAT         float64  `gorm:"default:0"         json:"vat"`
	UnitType    UnitType `gorm:"size:16"           json:"unitType"`
	Organic     bool     `gorm:"default:false"     json:"organic"`
	VariablePrice bool   `gorm:"default:false"     json:"variablePrice"`
	MultiWeight   bool   `gorm:"default:false"     json:"multiWeight"`
	HasFloatQt    bool   `gorm:"default:false"     json:"hasFloatQt"`
	Active        bool   `gorm:"default:true"      json:"active"`

	// Revente : indique que ce produit n'est pas produit par le Vendor du
	// catalogue mais simplement revendu. ResaleFrom est le nom (libre) du
	// producteur d'origine ; affiché tel quel sur la fiche produit.
	IsResale   bool    `gorm:"default:false" json:"isResale"`
	ResaleFrom *string `gorm:"size:128"      json:"resaleFrom,omitempty"`

	// Stocks
	Stock        *float64 `json:"stock,omitempty"`
	StockTracked bool     `gorm:"default:false" json:"stockTracked"`

	// Image (FK vers File)
	ImageID *uint `gorm:"column:imageId" json:"-"`
	Image   *File `gorm:"foreignKey:ImageID" json:"-"`

	// Catalogue parent
	CatalogID uint    `json:"-"`
	Catalog   Catalog `gorm:"foreignKey:CatalogID" json:"-"`

	// Catégorie (taxonomy)
	CategoryID *uint `json:"categoryId,omitempty"`

	// Lien vers la sous-catégorie de la taxonomie globale (TxpSubCategory).
	// La catégorie parente est dérivable via TxpSubCategory.CategoryID.
	// Nil → fallback "Autres / Tous" côté shop.
	TxpSubCategoryID *uint           `json:"-"`
	TxpSubCategory   *TxpSubCategory `gorm:"foreignKey:TxpSubCategoryID" json:"-"`
}

func (p *Product) TableName() string { return "products" }

// PriceHT retourne le prix hors taxe.
func (p *Product) PriceHT() float64 {
	if p.VAT == 0 {
		return p.Price
	}
	return p.Price / (1 + p.VAT/100)
}
