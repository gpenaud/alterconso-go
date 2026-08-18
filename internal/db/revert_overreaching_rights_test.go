package db

import "testing"

// La signature exacte laissee par l'elargissement errone, et elle seule.
func TestRevertRecognizesTheErroneousSignature(t *testing.T) {
	if !isOverreachingGrant(`[{"right":"Membership"},{"right":"CatalogAdmin"}]`) {
		t.Error("la signature de l'elargissement doit etre reconnue")
	}
}

// Les quatre appartenances legitimement corrigees portent aussi les deux
// delegations : elles gardent leurs droits.
func TestRevertSparesLegitimateCorrections(t *testing.T) {
	legitime := `[{"right":"Distributions"},{"right":"Parameters"},{"right":"Membership"},{"right":"CatalogAdmin"}]`
	if isOverreachingGrant(legitime) {
		t.Error("une correction legitime ne doit pas etre annulee")
	}
}

// Toute liste d'une autre forme a une autre origine.
func TestRevertIgnoresEverythingElse(t *testing.T) {
	for _, raw := range []string{
		``,
		`[]`,
		`[{"right":"Membership"}]`,
		`[{"right":"CatalogAdmin"},{"right":"Membership"}]`,
		`[{"right":"Membership"},{"right":"CatalogAdmin","params":["42"]}]`,
		`[{"right":"Membership"},{"right":"CatalogAdmin"},{"right":"Messages"}]`,
		`[{"right":"GroupAdmin"}]`,
		`pas du json`,
	} {
		if isOverreachingGrant(raw) {
			t.Errorf("%s ne devrait pas etre touche", raw)
		}
	}
}
