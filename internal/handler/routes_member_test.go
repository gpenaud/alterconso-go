package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
)

// Les routes des demandes d'adhésion voisinent avec /member/edit/:id et
// /member/view/:id. Le routeur de Gin refuse deux paramètres différents au même
// niveau d'arborescence et le fait savoir par une panique au démarrage —
// c'est-à-dire en production, après déploiement. Ce test la déclenche ici.
func TestMemberRoutesMountWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("montage des routes impossible : %v", p)
		}
	}()
	Register(r, nil, &config.Config{})

	want := map[string]string{
		"GET":  "/member/requests",
		"POST": "/member/requests/:id/:decision",
	}
	found := map[string]bool{}
	for _, route := range r.Routes() {
		if want[route.Method] == route.Path {
			found[route.Method] = true
		}
	}
	for method, path := range want {
		if !found[method] {
			t.Errorf("%s %s n'est pas enregistrée", method, path)
		}
	}
}

// L'inscription n'a qu'un chemin, et c'est celui qui demande le groupe puis
// confirme l'adresse. /api/user/register en ouvrait un second, sans l'un ni
// l'autre : un compte né par là n'aurait plus aucun moyen de rejoindre un
// groupe, et la route restait ouverte sans authentification.
func TestRegistrationHasASinglePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, nil, &config.Config{})

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}

	if seen["POST /api/user/register"] {
		t.Error("l'inscription par l'API de compatibilité contourne la demande d'adhésion")
	}
	if !seen["POST /user/register"] {
		t.Error("le formulaire d'inscription a disparu")
	}
}
