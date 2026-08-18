package model

import "encoding/json"

// Right représente un droit dans un groupe.
type Right string

const (
	RightGroupAdmin   Right = "GroupAdmin"   // responsable de groupe — exclusif
	RightCatalogAdmin Right = "CatalogAdmin" // peut avoir un catalogID optionnel
	RightMembership   Right = "Membership"
	RightMessages     Right = "Messages"

	// RightDistributions et RightParameters remplacent les anciens « droits
	// administrateur », qui ouvraient tout le groupe d'un bloc. Deux délégations
	// nettes valent mieux qu'un pouvoir général : on confie le calendrier des
	// distributions sans confier les paramètres du groupe, et réciproquement.
	//
	// Ni l'un ni l'autre ne fait de son porteur un responsable : l'accès complet
	// n'appartient qu'au responsable de groupe et au responsable technique.
	RightDistributions Right = "Distributions"
	RightParameters    Right = "Parameters"
)

// LabelTechnicalManager nomme le rôle du responsable technique de
// l'application. Ce n'est pas un droit de groupe et il ne s'attribue pas depuis
// l'interface : il tient à une adresse posée dans la configuration, unique pour
// toute l'installation. Il s'affiche à côté des droits d'un membre pour
// expliquer un accès que la liste de ses droits ne justifie pas.
const LabelTechnicalManager = "Responsable technique"

// RightLabels donne le nom de chaque droit tel qu'il s'affiche.
var RightLabels = map[Right]string{
	RightGroupAdmin:    "Responsable de groupe",
	RightCatalogAdmin:  "Gestion des catalogues",
	RightMembership:    "Gestion des membres",
	RightMessages:      "Messages",
	RightDistributions: "Gestion des distributions",
	RightParameters:    "Gestion des paramètres",
}

// AssignableRights énumère les droits attribuables depuis l'interface, dans
// l'ordre où ils s'affichent : le rôle de responsable d'abord, les délégations
// ensuite.
func AssignableRights() []Right {
	return []Right{
		RightGroupAdmin,
		RightDistributions,
		RightParameters,
		RightCatalogAdmin,
		RightMembership,
		RightMessages,
	}
}

// exclusiveRights : droits qu'un seul membre du groupe peut détenir à la fois.
//
// Les accorder à quelqu'un les retire à son prédécesseur — c'est un transfert,
// pas un cumul. Le responsable de groupe est désormais le seul dans ce cas : le
// rôle technique a quitté les droits de groupe pour la configuration, où il
// vaut pour toute l'installation.
var exclusiveRights = map[Right]bool{
	RightGroupAdmin: true,
}

// IsExclusiveRight indique si un droit n'admet qu'un titulaire par groupe.
func IsExclusiveRight(r Right) bool { return exclusiveRights[r] }

// ExclusiveRights énumère les droits à titulaire unique, dans un ordre stable.
func ExclusiveRights() []Right {
	return []Right{RightGroupAdmin}
}

// Label retourne le nom affichable d'un droit, ou sa valeur brute s'il n'en a
// pas — ce qui vaut mieux qu'une chaîne vide dans une interface.
func (r Right) Label() string {
	if l, ok := RightLabels[r]; ok {
		return l
	}
	return string(r)
}

// UserRight stocke un droit avec ses paramètres optionnels (ex: catalogID pour CatalogAdmin).
type UserRight struct {
	Right  Right    `json:"right"`
	Params []string `json:"params,omitempty"` // ex: ["42"] pour CatalogAdmin(42)
}

// UserGroup représente l'appartenance d'un user à un groupe, avec ses droits.
// Clé primaire composite (UserID, GroupID).
type UserGroup struct {
	UserID  uint  `gorm:"primaryKey" json:"userId"`
	GroupID uint  `gorm:"primaryKey" json:"groupId"`
	User    User  `gorm:"foreignKey:UserID" json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

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

// IsGroupManager : détient les pleins pouvoirs sur le groupe.
//
// Le responsable de groupe est désormais le seul dans ce cas côté groupe. Le
// responsable technique les a aussi, mais il ne se lit pas dans les droits d'un
// membre : son rôle vient de la configuration et vaut sur tous les groupes, si
// bien qu'il s'ajoute à ce test là où l'application le connaît — buildPageData
// et les gardes de routes — plutôt qu'ici.
//
// Les délégations « distributions » et « paramètres » ne donnent pas les pleins
// pouvoirs : c'est tout l'objet de leur découpage.
func (ug *UserGroup) IsGroupManager() bool {
	return ug.HasRight(RightGroupAdmin)
}

// CanManageRights : peut attribuer et retirer les droits des membres. Réservé
// au responsable du groupe, et au responsable technique par ailleurs.
func (ug *UserGroup) CanManageRights() bool {
	return ug.HasRight(RightGroupAdmin)
}

// IsGroupHead : responsable de groupe à proprement parler, celui qui porte le
// rôle unique et reçoit le courrier qui lui est adressé.
func (ug *UserGroup) IsGroupHead() bool {
	return ug.HasRight(RightGroupAdmin)
}

// CanAdminDatabase : accès à l'édition directe des tables.
func (ug *UserGroup) CanAdminDatabase() bool {
	return ug.HasRight(RightGroupAdmin)
}

// CanManageDistributions : tient le calendrier des distributions sans pour
// autant administrer le reste du groupe.
func (ug *UserGroup) CanManageDistributions() bool {
	return ug.HasRight(RightGroupAdmin) || ug.HasRight(RightDistributions)
}

// CanManageParameters : règle les paramètres du groupe — identité, adhésions,
// devise, documents — sans toucher au calendrier ni aux membres.
func (ug *UserGroup) CanManageParameters() bool {
	return ug.HasRight(RightGroupAdmin) || ug.HasRight(RightParameters)
}
