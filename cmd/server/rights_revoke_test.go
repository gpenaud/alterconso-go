package main

import "testing"

func TestRemoveRightDropsOnlyTheNamedOne(t *testing.T) {
	got, changed := removeRight(`[{"right":"GroupAdmin"},{"right":"Membership"}]`, "GroupAdmin")
	if !changed || got != `[{"right":"Membership"}]` {
		t.Errorf("obtenu %s (changed=%v)", got, changed)
	}
}

func TestRemoveRightKeepsParams(t *testing.T) {
	in := `[{"right":"GroupAdmin"},{"right":"CatalogAdmin","params":["42"]}]`
	got, _ := removeRight(in, "GroupAdmin")
	if got != `[{"right":"CatalogAdmin","params":["42"]}]` {
		t.Errorf("obtenu %s", got)
	}
}

func TestRemoveRightIgnoresAbsentOrGarbage(t *testing.T) {
	for _, raw := range []string{``, `[]`, `[{"right":"Membership"}]`, `pas du json`} {
		if _, changed := removeRight(raw, "GroupAdmin"); changed {
			t.Errorf("%s ne devrait pas changer", raw)
		}
	}
}

// Le dernier droit retire laisse une liste vide, pas une chaine vide : c'est
// l'etat qu'un adherent sans droits porte deja.
func TestRemoveRightLeavesEmptyList(t *testing.T) {
	got, changed := removeRight(`[{"right":"GroupAdmin"}]`, "GroupAdmin")
	if !changed || got != `[]` {
		t.Errorf("obtenu %q", got)
	}
}
