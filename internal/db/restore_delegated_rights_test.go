package db

import "testing"

// Un « Administration » converti se reconnait aux deux delegations ensemble :
// ses porteurs retrouvent les membres et les catalogues.
func TestRestoreTargetsConvertedAdministration(t *testing.T) {
	got, changed := restoreMembersAndCatalogs(`[{"right":"Distributions"},{"right":"Parameters"}]`)
	if !changed {
		t.Fatal("correction attendue")
	}
	want := `[{"right":"Distributions"},{"right":"Parameters"},{"right":"Membership"},{"right":"CatalogAdmin"}]`
	if got != want {
		t.Errorf("obtenu %s", got)
	}
}

// Un « DatabaseAdmin » seul n'a rien laisse derriere lui : la liste vide est sa
// signature, qu'un membre jamais dote ne porte pas — sa colonne est vide.
func TestRestoreTargetsEmptiedRights(t *testing.T) {
	if _, changed := restoreMembersAndCatalogs(`[]`); !changed {
		t.Error("une liste vide doit etre corrigee")
	}
	if _, changed := restoreMembersAndCatalogs(``); changed {
		t.Error("un membre jamais dote ne doit pas recevoir de droits")
	}
}

// Une delegation seule n'est pas une conversion : on ne rend rien a qui n'a
// jamais eu le pouvoir general.
func TestRestoreIgnoresSingleDelegation(t *testing.T) {
	for _, raw := range []string{
		`[{"right":"Distributions"}]`,
		`[{"right":"Parameters"}]`,
		`[{"right":"Membership"}]`,
		`[{"right":"GroupAdmin"}]`,
	} {
		if _, changed := restoreMembersAndCatalogs(raw); changed {
			t.Errorf("%s ne devrait pas etre touche", raw)
		}
	}
}

// Un droit de catalogue restreint survit : l'elargir a tous depasserait la
// reparation.
func TestRestoreKeepsRestrictedCatalogs(t *testing.T) {
	in := `[{"right":"Distributions"},{"right":"Parameters"},{"right":"CatalogAdmin","params":["42"]}]`
	got, changed := restoreMembersAndCatalogs(in)
	if !changed {
		t.Fatal("les membres restent a rendre")
	}
	want := `[{"right":"Distributions"},{"right":"Parameters"},{"right":"CatalogAdmin","params":["42"]},{"right":"Membership"}]`
	if got != want {
		t.Errorf("obtenu %s", got)
	}
}

// Deux passages ne doivent pas produire de doublon.
func TestRestoreIsStable(t *testing.T) {
	once, _ := restoreMembersAndCatalogs(`[{"right":"Distributions"},{"right":"Parameters"}]`)
	twice, changed := restoreMembersAndCatalogs(once)
	if changed || twice != once {
		t.Errorf("seconde passe instable : %s puis %s", once, twice)
	}
}
