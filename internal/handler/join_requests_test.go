package handler

import (
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// Le courrier part vers ceux qui peuvent y répondre. Prévenir une délégation
// qui n'a pas la main sur les membres ne ferait que lui demander un geste
// qu'elle ne peut pas accomplir.
func TestOnlyMemberManagersAreNotified(t *testing.T) {
	cases := []struct {
		nom  string
		ug   *model.UserGroup
		want bool
	}{
		{"gestion des membres", ug(`[{"right":"Membership"}]`), true},
		{"responsable de groupe", ug(`[{"right":"GroupAdmin"}]`), true},
		{"distributions", ug(`[{"right":"Distributions"}]`), false},
		{"paramètres", ug(`[{"right":"Parameters"}]`), false},
		{"catalogues", ug(`[{"right":"CatalogAdmin"}]`), false},
		{"messages", ug(`[{"right":"Messages"}]`), false},
		{"adhérent sans droit", ug(`[]`), false},
		{"non-membre", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := receivesJoinRequests(tc.ug); got != tc.want {
				t.Errorf("obtenu %v, attendu %v", got, tc.want)
			}
		})
	}
}

// Le bandeau se compose côté serveur à partir d'un code. Recopier le texte
// depuis la barre d'adresse laisserait écrire n'importe quelle phrase dans un
// encadré que l'application signe de son nom.
func TestFlashComesFromACodeNotFromTheURL(t *testing.T) {
	msg, isError := joinRequestFlash("accepted", "DUPONT Marie")
	if !strings.Contains(msg, "DUPONT Marie") || !strings.Contains(msg, "membre du groupe") {
		t.Errorf("acceptation mal annoncée : %q", msg)
	}
	if isError {
		t.Error("une acceptation n'est pas une erreur")
	}

	if msg, isError := joinRequestFlash("failed", ""); !isError || msg == "" {
		t.Errorf("un échec doit se signaler comme tel : %q (erreur=%v)", msg, isError)
	}

	// Un code inconnu ne rend aucun bandeau : c'est ce qui empêche une phrase
	// forgée d'apparaître.
	if msg, _ := joinRequestFlash("Votre compte a été suspendu, appelez le", ""); msg != "" {
		t.Errorf("un code inconnu ne doit rien afficher, obtenu %q", msg)
	}
}

// Le nom vient de la base, mais transite par l'URL : un nom démesuré y
// déformerait le bandeau.
func TestFlashNameStaysBounded(t *testing.T) {
	msg, _ := joinRequestFlash("accepted", strings.Repeat("a", 500))
	if len(msg) > 200 {
		t.Errorf("bandeau de %d octets, trop long", len(msg))
	}
}

// Accepter fait entrer quelqu'un dans un groupe : l'action passe par un
// formulaire POST. En lien GET, le préchargement d'un navigateur ou d'un
// antivirus de messagerie suffirait à décider à la place du gestionnaire.
func TestDecisionsAreNotClickableLinks(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html",
		"cycles_style.html", "member_requests.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	data := JoinRequestsData{PageData: PageData{
		Title:         "Demandes d'adhésion",
		User:          &model.User{ID: 1, FirstName: "Alice"},
		Group:         &model.Group{ID: 1, Name: "AMAP"},
		HasMembership: true,
	}}
	data.Requests = []JoinRequestEntry{{
		ID: 7, UserID: 42, Name: "DUPONT Marie",
		Email: "marie@example.org", Date: "01/09/2026",
	}}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", data); err != nil {
		t.Fatalf("render : %v", err)
	}
	out := sb.String()

	for _, attendu := range []string{
		`action="/member/requests/7/accept"`,
		`action="/member/requests/7/refuse"`,
		`method="POST"`,
		"DUPONT Marie",
	} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%s absent de l'écran", attendu)
		}
	}
	if strings.Contains(out, `href="/member/requests/7/accept"`) {
		t.Error("la décision ne doit pas être un lien")
	}
}

// L'inscription demande le groupe : sans lui, le compte créé n'a personne à
// qui adresser sa demande.
func TestRegistrationAsksForAGroup(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "cycles_style.html", "register.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	data := RegisterData{Step: 1, OpenGroups: []model.Group{
		{ID: 3, Name: "AMAP du Pré"},
		{ID: 8, Name: "Les Paniers du Coin"},
	}}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", data); err != nil {
		t.Fatalf("render : %v", err)
	}
	out := sb.String()

	for _, attendu := range []string{
		`name="groupId"`, `value="3"`, "AMAP du Pré", "Les Paniers du Coin",
		`name="message"`,
	} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%s absent du formulaire d'inscription", attendu)
		}
	}
}

// Aucun groupe n'accueille : le formulaire le dit plutôt que d'afficher une
// liste déroulante vide qu'aucune saisie ne peut satisfaire.
func TestRegistrationSaysWhenNoGroupWelcomes(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "cycles_style.html", "register.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", RegisterData{Step: 1}); err != nil {
		t.Fatalf("render : %v", err)
	}
	if !strings.Contains(sb.String(), "Aucun groupe n'accepte d'inscription") {
		t.Error("le formulaire devrait annoncer qu'aucun groupe n'accueille")
	}
}

// Le candidat dont la demande est partie ne doit pas lire « vous n'appartenez à
// aucun groupe » sans autre explication : cela se lit comme un échec.
func TestChooseScreenShowsPendingRequests(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "choose.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	var sb strings.Builder
	err = tpl.ExecuteTemplate(&sb, "base", PageData{
		Title:         "Choisir un groupe",
		User:          &model.User{ID: 4, FirstName: "Marie"},
		HideNav:       true,
		PendingGroups: []model.Group{{ID: 3, Name: "AMAP du Pré"}},
	})
	if err != nil {
		t.Fatalf("render : %v", err)
	}
	out := sb.String()

	if !strings.Contains(out, "AMAP du Pré") {
		t.Error("le groupe demandé devrait être nommé")
	}
	if !strings.Contains(out, "Demande") {
		t.Error("l'écran devrait annoncer la demande en cours")
	}
	// Les deux messages ensemble se contredisent : « vous n'appartenez à aucun
	// groupe » annule ce que « demande en cours » vient d'annoncer.
	if strings.Contains(out, "Vous n'appartenez à aucun groupe") {
		t.Error("le constat d'absence ne doit pas doubler l'annonce de la demande")
	}
}

// Le lien reçu par courrier passe par l'écran de choix de groupe quand la
// session n'en porte pas encore. Il doit y survivre — sinon le gestionnaire
// atterrit sur l'accueil et doit retrouver seul l'écran qu'on lui montrait.
func TestMailLinkSurvivesTheGroupChoice(t *testing.T) {
	if got := safeRedirectPath("/member/requests"); got != "/member/requests" {
		t.Errorf("destination interne perdue : %q", got)
	}

	// Et il ne doit pas devenir un tremplin : « //ailleurs.example » est une
	// URL absolue pour un navigateur.
	for _, hostile := range []string{
		"//ailleurs.example/piege",
		"https://ailleurs.example",
		"ailleurs.example",
		"",
	} {
		if got := safeRedirectPath(hostile); got != "" {
			t.Errorf("%q aurait dû être écarté, obtenu %q", hostile, got)
		}
	}
}
