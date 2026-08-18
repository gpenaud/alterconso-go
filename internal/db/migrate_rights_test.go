package db

import "testing"

// « Administration » ouvrait tout le groupe : il se convertit dans les deux
// delegations qui le remplacent, sans rien retirer a son porteur.
func TestMigrationSplitsAdministration(t *testing.T) {
	got, changed := migrateRightsJSON(`[{"right":"Administration"}]`)
	if !changed {
		t.Fatal("conversion attendue")
	}
	want := `[{"right":"Distributions"},{"right":"Parameters"}]`
	if got != want {
		t.Errorf("obtenu %s, attendu %s", got, want)
	}
}

// Le role technique est passe en configuration : le droit de groupe qui le
// portait disparait, sans quoi d'anciens porteurs garderaient la base ouverte.
func TestMigrationDropsDatabaseAdmin(t *testing.T) {
	got, changed := migrateRightsJSON(`[{"right":"DatabaseAdmin"},{"right":"Membership"}]`)
	if !changed {
		t.Fatal("conversion attendue")
	}
	if got != `[{"right":"Membership"}]` {
		t.Errorf("obtenu %s", got)
	}
}

// Les parametres d'un droit de catalogue survivent a la conversion.
func TestMigrationKeepsParams(t *testing.T) {
	in := `[{"right":"CatalogAdmin","params":["42"]}]`
	got, changed := migrateRightsJSON(in)
	if changed || got != in {
		t.Errorf("droit intact attendu, obtenu %s (changed=%v)", got, changed)
	}
}

// Une seconde execution ne doit plus rien trouver a convertir.
func TestMigrationIsIdempotent(t *testing.T) {
	once, _ := migrateRightsJSON(`[{"right":"Administration"},{"right":"DatabaseAdmin"}]`)
	twice, changed := migrateRightsJSON(once)
	if changed || twice != once {
		t.Errorf("seconde passe instable : %s puis %s", once, twice)
	}
}

// Un porteur cumulant l'ancien pouvoir general et une delegation deja accordee
// ne doit pas ressortir avec le meme droit deux fois.
func TestMigrationAvoidsDuplicates(t *testing.T) {
	got, _ := migrateRightsJSON(`[{"right":"Distributions"},{"right":"Administration"}]`)
	if got != `[{"right":"Distributions"},{"right":"Parameters"}]` {
		t.Errorf("obtenu %s", got)
	}
}

// Une valeur illisible est laissee telle quelle : on n'efface pas un droit
// qu'on ne comprend pas.
func TestMigrationLeavesGarbageAlone(t *testing.T) {
	got, changed := migrateRightsJSON(`pas du json`)
	if changed || got != `pas du json` {
		t.Errorf("valeur modifiee : %s", got)
	}
}
