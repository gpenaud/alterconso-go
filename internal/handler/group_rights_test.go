package handler

import (
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// Les deux delegations qui ont remplace les « droits administrateur »
// n'ouvrent qu'un ecran chacune. C'est tout l'objet du decoupage : confier le
// calendrier sans confier les reglages du groupe, et reciproquement.
func TestDelegationsStayNarrow(t *testing.T) {
	distributions := ug(`[{"right":"Distributions"}]`)
	parametres := ug(`[{"right":"Parameters"}]`)

	cases := []struct {
		nom  string
		got  bool
		want bool
	}{
		{"distributions ⇒ calendrier", distributions.CanManageDistributions(), true},
		{"distributions ⇏ parametres", distributions.CanManageParameters(), false},
		{"distributions ⇏ pleins pouvoirs", distributions.IsGroupManager(), false},
		{"distributions ⇏ base de donnees", distributions.CanAdminDatabase(), false},
		{"distributions ⇏ attribution des droits", distributions.CanManageRights(), false},

		{"parametres ⇒ reglages", parametres.CanManageParameters(), true},
		{"parametres ⇏ calendrier", parametres.CanManageDistributions(), false},
		{"parametres ⇏ pleins pouvoirs", parametres.IsGroupManager(), false},
		{"parametres ⇏ base de donnees", parametres.CanAdminDatabase(), false},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("obtenu %v, attendu %v", tc.got, tc.want)
			}
		})
	}
}

// Le responsable de groupe a tout : les delegations lui sont acquises sans
// qu'on ait a les lui accorder une a une.
func TestGroupHeadHoldsEverything(t *testing.T) {
	head := ug(`[{"right":"GroupAdmin"}]`)

	for nom, got := range map[string]bool{
		"pleins pouvoirs":          head.IsGroupManager(),
		"calendrier":               head.CanManageDistributions(),
		"parametres":               head.CanManageParameters(),
		"base de donnees":          head.CanAdminDatabase(),
		"attribution des droits":   head.CanManageRights(),
		"responsable a proprement": head.IsGroupHead(),
	} {
		if !got {
			t.Errorf("%s : refuse au responsable de groupe", nom)
		}
	}
}

// Seul le porteur de GroupAdmin est responsable : c'est ce test que groupHeads
// applique a chaque membre pour dresser la liste des destinataires.
func TestGroupHeadSelection(t *testing.T) {
	cases := []struct {
		nom  string
		ug   *model.UserGroup
		want bool
	}{
		{"responsable de groupe", ug(`[{"right":"GroupAdmin"}]`), true},
		{"gestion des distributions", ug(`[{"right":"Distributions"}]`), false},
		{"gestion des parametres", ug(`[{"right":"Parameters"}]`), false},
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

// Qui attribue les droits : le responsable du groupe, et le responsable
// technique par loadGroupAccess, qui lui pose GroupAdmin sur tous les groupes.
func TestWhoCanManageRights(t *testing.T) {
	cases := []struct {
		nom  string
		ug   *model.UserGroup
		want bool
	}{
		{"responsable de groupe", ug(`[{"right":"GroupAdmin"}]`), true},
		{"gestion des distributions", ug(`[{"right":"Distributions"}]`), false},
		{"gestion des parametres", ug(`[{"right":"Parameters"}]`), false},
		{"gestion des membres", ug(`[{"right":"Membership"}]`), false},
		{"simple membre", ug(`[]`), false},
		{"responsable technique", ug(fullRightsJSON), true},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := tc.ug.CanManageRights(); got != tc.want {
				t.Errorf("CanManageRights = %v, attendu %v", got, tc.want)
			}
		})
	}
}

// Les destinataires du courrier adresse aux responsables : le responsable du
// groupe et le responsable technique, personne d'autre.
func TestManagerRecipientEmails(t *testing.T) {
	head := model.UserGroup{User: model.User{Email: "responsable@exemple.fr"}}

	got := managerRecipientEmails(nil, []model.UserGroup{head}, []string{"technique@exemple.fr"})
	want := []string{"responsable@exemple.fr", "technique@exemple.fr"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("attendu %v, obtenu %v", want, got)
	}

	// Le responsable technique est souvent aussi responsable d'un groupe : une
	// seule fois.
	deux := managerRecipientEmails(nil,
		[]model.UserGroup{{User: model.User{Email: "technique@exemple.fr"}}},
		[]string{"technique@exemple.fr"},
	)
	if len(deux) != 1 {
		t.Errorf("le cumul des deux roles doit donner un destinataire, obtenu %v", deux)
	}

	// Un membre sans email ne produit pas de destinataire vide, qui ferait
	// echouer l'envoi sans rien dire.
	sansEmail := managerRecipientEmails(nil, []model.UserGroup{{User: model.User{}}}, nil)
	if len(sansEmail) != 0 {
		t.Errorf("attendu aucun destinataire, obtenu %v", sansEmail)
	}
}

// L'adresse declaree par le groupe l'emporte sur celle du membre qui porte le
// role : une adresse de fonction survit au changement de responsable.
func TestGroupHeadEmailPrefersDeclaredAddress(t *testing.T) {
	declared := "contact@valdebrenne.fr"
	g := &model.Group{HeadEmail: &declared}
	heads := []model.UserGroup{{User: model.User{Email: "perso@exemple.fr"}}}

	got := groupHeadEmail(g, heads)
	if len(got) != 1 || got[0] != declared {
		t.Errorf("attendu [%s], obtenu %v", declared, got)
	}

	// Non renseignee, c'est l'adresse personnelle qui sert — et non l'absence
	// de tout destinataire.
	if got := groupHeadEmail(&model.Group{}, heads); len(got) != 1 || got[0] != "perso@exemple.fr" {
		t.Errorf("repli sur l'adresse du responsable attendu, obtenu %v", got)
	}

	// Une valeur blanche vaut une absence.
	blanche := "   "
	if got := groupHeadEmail(&model.Group{HeadEmail: &blanche}, heads); len(got) != 1 || got[0] != "perso@exemple.fr" {
		t.Errorf("adresse blanche : repli attendu, obtenu %v", got)
	}
}

// Le role technique se lit dans la configuration, jamais en base : une adresse
// vide ne fait de personne un responsable technique.
func TestTechnicalManagerRecognition(t *testing.T) {
	SetTechnicalManager("")
	if isTechnicalManagerEmail("qui@exemple.fr") {
		t.Error("aucune adresse configuree : personne ne tient le role")
	}

	SetTechnicalManager("Technique@Exemple.FR")
	defer SetTechnicalManager("")

	if !isTechnicalManagerEmail("technique@exemple.fr") {
		t.Error("la comparaison doit ignorer la casse")
	}
	if !isTechnicalManagerEmail("  technique@exemple.fr  ") {
		t.Error("les espaces autour de l'adresse ne doivent pas compter")
	}
	if isTechnicalManagerEmail("autre@exemple.fr") {
		t.Error("une autre adresse ne tient pas le role")
	}
}

// Le compte d'installation porte presque toujours « GroupAdmin » en base. Il
// ne doit pas pour autant figurer comme responsable du groupe : son role vaut
// pour tous les groupes, et il a sa propre entree dans la messagerie.
func TestTechnicalManagerIsNotAGroupHead(t *testing.T) {
	SetTechnicalManager("alterconso@leportail.org")
	defer SetTechnicalManager("")

	membres := []model.UserGroup{
		{User: model.User{Email: "alterconso@leportail.org"}, Rights: `[{"right":"GroupAdmin"}]`},
		{User: model.User{Email: "responsable@exemple.fr"}, Rights: `[{"right":"GroupAdmin"}]`},
		{User: model.User{Email: "adherent@exemple.fr"}, Rights: `[]`},
	}

	heads := selectGroupHeads(membres)
	if len(heads) != 1 || heads[0].User.Email != "responsable@exemple.fr" {
		t.Fatalf("attendu le seul responsable du groupe, obtenu %v", heads)
	}

	// Et donc pas davantage derriere l'entree « responsable du groupe » de la
	// messagerie, qui part de cette liste.
	got := managerRecipientEmails(nil, heads, nil)
	if len(got) != 1 || got[0] != "responsable@exemple.fr" {
		t.Errorf("attendu le seul responsable du groupe, obtenu %v", got)
	}

	// Sans adresse configuree, plus personne n'est ecarte : le droit en base
	// reprend effet.
	SetTechnicalManager("")
	if len(selectGroupHeads(membres)) != 2 {
		t.Error("les deux porteurs de GroupAdmin devraient revenir")
	}
}

// Le role de responsable s'attribue par le responsable technique : c'est lui
// qui le designe a l'ouverture d'un groupe, et lui seul qui le deplace ensuite.
// Le responsable en place distribue les autres droits, mais ne transfere pas
// le sien.
func TestGroupHeadRoleIsAssignedByTechnicalManagerOnly(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "amapadmin_layout.html", "amapadmin_rights_edit.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	// Vu par le responsable du groupe : le role s'affiche, sans case a cocher.
	var parLeResponsable strings.Builder
	if err := tpl.ExecuteTemplate(&parLeResponsable, "content", AmapAdminRightsEditData{
		HasGroupAdmin: true, CanAssignGroupHead: false,
	}); err != nil {
		t.Fatalf("render : %v", err)
	}
	if strings.Contains(parLeResponsable.String(), `name="right_group_admin"`) {
		t.Error("le responsable du groupe ne doit pas pouvoir transferer son role")
	}
	if !strings.Contains(parLeResponsable.String(), "attribué par le responsable technique") {
		t.Error("l'ecran devrait dire a qui revient l'attribution")
	}

	// Vu par le responsable technique : la case est la.
	var parLeTechnique strings.Builder
	if err := tpl.ExecuteTemplate(&parLeTechnique, "content", AmapAdminRightsEditData{
		CanAssignGroupHead: true,
	}); err != nil {
		t.Fatalf("render : %v", err)
	}
	if !strings.Contains(parLeTechnique.String(), `name="right_group_admin"`) {
		t.Error("le responsable technique doit pouvoir designer le responsable")
	}
}
