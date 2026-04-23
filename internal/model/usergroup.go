package model

import "encoding/json"

// Right représente un droit dans un groupe.
type Right string

const (
	RightGroupAdmin    Right = "GroupAdmin"
	RightCatalogAdmin  Right = "CatalogAdmin" // peut avoir un catalogID optionnel
	RightMembership    Right = "Membership"
	RightMessages      Right = "Messages"
)

// UserRight stocke un droit avec ses paramètres optionnels (ex: catalogID pour CatalogAdmin).
type UserRight struct {
	Right  Right    `json:"right"`
	Params []string `json:"params,omitempty"` // ex: ["42"] pour CatalogAdmin(42)
}

// UserGroup représente l'appartenance d'un user à un groupe, avec ses droits.
// Clé primaire composite (UserID, GroupID).
type UserGroup struct {
	UserID  uint    `gorm:"primaryKey" json:"userId"`
	GroupID uint    `gorm:"primaryKey" json:"groupId"`
	User    User    `gorm:"foreignKey:UserID" json:"-"`
	Group   Group   `gorm:"foreignKey:GroupID" json:"-"`

	// Balance du compte dans la devise du groupe
	Balance float64 `gorm:"default:0" json:"balance"`

	// Droits sérialisés en JSON : [{"right":"GroupAdmin"},{"right":"CatalogAdmin","params":["42"]}]
	Rights string `gorm:"type:text" json:"-"`
}

func (ug *UserGroup) TableName() string { return "user_groups" }

// HasRight vérifie si l'utilisateur possède un droit spécifique dans ce groupe.
// Si catalogID est passé pour CatalogAdmin, vérifie aussi le droit global (params nil).
func (ug *UserGroup) HasRight(r Right, params ...string) bool {
	rights := ug.GetRights()
	for _, right := range rights {
		if right.Right != r {
			continue
		}
		if len(params) == 0 {
			return true
		}
		// Droit global (aucun paramètre) → accès à tout
		if right.Params == nil {
			return true
		}
		// Droit spécifique → vérifier le paramètre
		for _, p := range right.Params {
			for _, want := range params {
				if p == want {
					return true
				}
			}
		}
	}
	return false
}

// GetRights désérialise les droits depuis le JSON.
func (ug *UserGroup) GetRights() []UserRight {
	if ug.Rights == "" {
		return nil
	}
	var rights []UserRight
	if err := json.Unmarshal([]byte(ug.Rights), &rights); err != nil {
		return nil
	}
	return rights
}

func (ug *UserGroup) IsGroupManager() bool {
	return ug.HasRight(RightGroupAdmin)
}
