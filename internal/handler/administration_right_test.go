package handler

import (
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// Les « droits administrateur » ouvrent tout, sauf la base de données.
func TestAdministrationRightScope(t *testing.T) {
	admin := ug(`[{"right":"Administration"}]`)
	responsable := ug(`[{"right":"GroupAdmin"}]`)
	technique := ug(`[{"right":"DatabaseAdmin"}]`)
	adminEtTechnique := ug(`[{"right":"Administration"},{"right":"DatabaseAdmin"}]`)

	cases := []struct {
		nom    string
		ug     *model.UserGroup
		rights []model.Right
		want   bool
	}{
		// Une liste vide exige les pleins pouvoirs : les droits
		// administrateur les donnent.
		{"administration ⇒ pages réservées aux responsables", admin, nil, true},
		{"administration ⇒ catalogues", admin, []model.Right{model.RightCatalogAdmin}, true},
		{"administration ⇒ membres", admin, []model.Right{model.RightMembership}, true},

		// La seule porte fermée.
		{"administration ⇏ base de données", admin, []model.Right{model.RightDatabaseAdmin}, false},
		{"responsable de groupe ⇒ base de données", responsable, []model.Right{model.RightDatabaseAdmin}, true},
		{"responsable technique ⇒ base de données", technique, []model.Right{model.RightDatabaseAdmin}, true},
		{"les deux cumulés ⇒ base de données", adminEtTechnique, []model.Right{model.RightDatabaseAdmin}, true},

		// Le responsable technique a les mêmes droits que le responsable de
		// groupe : les pages d'administration lui sont ouvertes aussi.
		{"technique ⇒ pages réservées aux responsables", technique, nil, true},
		{"technique ⇒ catalogues", technique, []model.Right{model.RightCatalogAdmin}, true},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := authorize(tc.ug, tc.rights); got != tc.want {
				t.Errorf("authorize = %v, attendu %v", got, tc.want)
			}
		})
	}
}

// La distinction entre « pleins pouvoirs » et « responsable de groupe » : seul
// le second porte le rôle unique et reçoit les messages qui lui sont adressés.
func TestAdministrationIsNotGroupHead(t *testing.T) {
	admin := ug(`[{"right":"Administration"}]`)

	if !admin.IsGroupManager() {
		t.Error("les droits administrateur devraient donner les pleins pouvoirs")
	}
	if admin.IsGroupHead() {
		t.Error("ils ne font pas de leur porteur le responsable du groupe")
	}
	if admin.CanAdminDatabase() {
		t.Error("ils ne doivent pas ouvrir la base de données")
	}

}

// Les destinataires du courrier adressé aux responsables : le responsable du
// groupe et le super-administrateur, personne d'autre.
func TestManagerRecipientEmails(t *testing.T) {
	head := model.UserGroup{User: model.User{Email: "responsable@exemple.fr"}}
	superAdmin := model.User{Email: "admin@exemple.fr"}

	got := managerRecipientEmails([]model.UserGroup{head}, []model.User{superAdmin})
	want := []string{"responsable@exemple.fr", "admin@exemple.fr"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("attendu %v, obtenu %v", want, got)
	}

	// Le compte par défaut cumule souvent les deux rôles : une seule fois.
	deux := managerRecipientEmails(
		[]model.UserGroup{{User: model.User{Email: "admin@exemple.fr"}}},
		[]model.User{superAdmin},
	)
	if len(deux) != 1 {
		t.Errorf("le cumul des deux rôles doit donner un destinataire, obtenu %v", deux)
	}

	// Un membre sans email ne produit pas de destinataire vide, qui ferait
	// échouer l'envoi sans rien dire.
	sansEmail := managerRecipientEmails([]model.UserGroup{{User: model.User{}}}, nil)
	if len(sansEmail) != 0 {
		t.Errorf("attendu aucun destinataire, obtenu %v", sansEmail)
	}
}

// Seul le porteur de GroupAdmin est responsable : c'est ce test que groupHeads
// applique à chaque membre pour dresser la liste des destinataires.
func TestGroupHeadSelection(t *testing.T) {
	cases := []struct {
		nom  string
		ug   *model.UserGroup
		want bool
	}{
		{"responsable de groupe", ug(`[{"right":"GroupAdmin"}]`), true},
		{"droits administrateur", ug(`[{"right":"Administration"}]`), false},
		{"responsable technique", ug(`[{"right":"DatabaseAdmin"}]`), false},
		{"gestion des catalogues", ug(`[{"right":"CatalogAdmin"}]`), false},
		{"simple membre", ug(`[]`), false},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := tc.ug.IsGroupHead(); got != tc.want {
				t.Errorf("IsGroupHead = %v, attendu %v", got, tc.want)
			}
		})
	}
}

// Qui attribue les droits : les deux responsables, et personne d'autre.
//
// C'est la porte qui rend la restriction sur la base de données effective —
// sans elle, un porteur des droits administrateur se désignerait responsable
// technique et l'ouvrirait lui-même.
func TestWhoCanManageRights(t *testing.T) {
	cases := []struct {
		nom  string
		ug   *model.UserGroup
		want bool
	}{
		{"responsable de groupe", ug(`[{"right":"GroupAdmin"}]`), true},
		{"responsable technique", ug(`[{"right":"DatabaseAdmin"}]`), true},
		{"droits administrateur", ug(`[{"right":"Administration"}]`), false},
		{"gestion des membres", ug(`[{"right":"Membership"}]`), false},
		{"simple membre", ug(`[]`), false},
		// Le superadmin passe par loadGroupAccess, qui lui pose GroupAdmin sur
		// tous les groupes : c'est ce que reproduit ce cas.
		{"super-administrateur", ug(fullRightsJSON), true},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := tc.ug.CanManageRights(); got != tc.want {
				t.Errorf("CanManageRights = %v, attendu %v", got, tc.want)
			}
		})
	}
}
