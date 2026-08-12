package model

import "encoding/json"

// Right représente un droit dans un groupe.
type Right string

const (
	RightGroupAdmin    Right = "GroupAdmin"   // responsable de groupe — exclusif
	RightCatalogAdmin  Right = "CatalogAdmin" // peut avoir un catalogID optionnel
	RightMembership    Right = "Membership"
	RightMessages      Right = "Messages"
	RightDatabaseAdmin Right = "DatabaseAdmin" // responsable technique — exclusif

	// RightAdministration : tous les pouvoirs d'administration du groupe, hors
	// base de données. Cumulable, contrairement aux deux rôles de responsable.
	//
	// Son nom ne contient pas « GroupAdmin », et ce n'est pas un détail : les
	// destinataires des messages adressés aux responsables se choisissent par
	// `rights LIKE '%GroupAdmin%'`. Un nom qui contiendrait cette chaîne ferait
	// entrer ses porteurs dans cette liste, ce qu'on ne veut pas.
	RightAdministration Right = "Administration"
)

// LabelSuperAdmin nomme le rôle du compte administrateur de l'application —
// celui que garantit EnsureSuperAdmin au démarrage. Il n'est pas un droit de
// groupe : il porte sur TOUS les groupes à la fois, et ne se cumule ni ne se
// transfère comme les autres. Le compte par défaut détient souvent en plus un
// « GroupAdmin » hérité de l'installation, qu'il ne faut pas afficher à sa
// place : cela le ferait passer pour le responsable d'un groupe en particulier.
const LabelSuperAdmin = "Super-administrateur"

// RightLabels donne le nom de chaque droit tel qu'il s'affiche.
var RightLabels = map[Right]string{
	RightGroupAdmin:     "Responsable de groupe",
	RightDatabaseAdmin:  "Responsable technique",
	RightAdministration: "Droits administrateur",
	RightCatalogAdmin:   "Gestion des catalogues",
	RightMembership:     "Gestion des membres",
	RightMessages:       "Messages",
}

// exclusiveRights : droits qu'un seul membre du groupe peut détenir à la fois.
//
// Les accorder à quelqu'un les retire à son prédécesseur — c'est un transfert,
// pas un cumul. Le superadmin global n'entre pas dans ce compte : il tient ses
// droits d'ailleurs, sur tous les groupes à la fois.
var exclusiveRights = map[Right]bool{
	RightGroupAdmin:    true,
	RightDatabaseAdmin: true,
}

// IsExclusiveRight indique si un droit n'admet qu'un titulaire par groupe.
func IsExclusiveRight(r Right) bool { return exclusiveRights[r] }

// ExclusiveRights énumère les droits à titulaire unique, dans un ordre stable.
func ExclusiveRights() []Right {
	return []Right{RightGroupAdmin, RightDatabaseAdmin}
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

// IsGroupManager : détient les pleins pouvoirs d'administration du groupe.
//
// Vrai pour les trois : responsable de groupe, responsable technique — qui a
// les mêmes droits que lui — et porteur des « droits administrateur », dont
// l'accès s'arrête à la base de données et à l'attribution des droits. Ces deux
// portes-là ne s'ouvrent pas ici, mais par CanAdminDatabase et CanManageRights.
//
// Une dizaine de handlers appellent cette méthode en direct : c'est le point de
// passage qu'il faut élargir, plutôt que de les reprendre un à un.
func (ug *UserGroup) IsGroupManager() bool {
	return ug.HasRight(RightGroupAdmin) ||
		ug.HasRight(RightDatabaseAdmin) ||
		ug.HasRight(RightAdministration)
}

// CanManageRights : peut attribuer et retirer les droits des membres.
//
// Fermé aux « droits administrateur » : sans quoi leur porteur s'accorderait le
// rôle technique, et par lui la base de données que ce droit lui refuse. La
// restriction ne tiendrait qu'aussi longtemps qu'il ne chercherait pas à la
// contourner.
func (ug *UserGroup) CanManageRights() bool {
	return ug.HasRight(RightGroupAdmin) || ug.HasRight(RightDatabaseAdmin)
}

// IsGroupHead : responsable de groupe à proprement parler, celui qui porte le
// rôle unique. À distinguer de IsGroupManager, plus large.
func (ug *UserGroup) IsGroupHead() bool {
	return ug.HasRight(RightGroupAdmin)
}

// CanAdminDatabase : accès à l'édition directe des tables.
//
// Les « droits administrateur » s'arrêtent à cette porte, comme à celle de
// CanManageRights — les deux seules que le responsable de groupe et le
// responsable technique ne partagent pas avec eux.
func (ug *UserGroup) CanAdminDatabase() bool {
	return ug.HasRight(RightGroupAdmin) || ug.HasRight(RightDatabaseAdmin)
}
