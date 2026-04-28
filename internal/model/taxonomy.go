package model

// TxpCategory : catégorie globale de la taxonomie produits (transverse à tous
// les groupes). Hérité du schéma Haxe (table `TxpCategory`), nom GORM
// snake_case = `txp_categories`. Référencée par TxpSubCategory et lue par
// le handler ShopCategories pour le shop legacy.
type TxpCategory struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string `gorm:"size:128;not null"        json:"name"`
	Image        string `gorm:"size:64"                  json:"image"` // basename sans extension, ex "fruits-legumes"
	DisplayOrder int    `gorm:"default:0"                json:"displayOrder"`

	SubCategories []TxpSubCategory `gorm:"foreignKey:CategoryID" json:"subcategories"`
}

func (TxpCategory) TableName() string { return "txp_categories" }

// TxpSubCategory : sous-catégorie d'une TxpCategory.
type TxpSubCategory struct {
	ID         uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string `gorm:"size:128;not null"        json:"name"`
	CategoryID uint   `gorm:"index"                    json:"-"`
	Category   TxpCategory `gorm:"foreignKey:CategoryID" json:"-"`
}

func (TxpSubCategory) TableName() string { return "txp_sub_categories" }
