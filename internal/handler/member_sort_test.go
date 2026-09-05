package handler

import (
	"sort"
	"testing"
)

// Les listes qui circulent en distribution se lisent dans l'ordre alphabétique
// des noms de famille — pas des prénoms, pas des id.
func TestMemberSortKeyOrdersByLastName(t *testing.T) {
	membres := []struct{ prenom, nom string }{
		{"Zoé", "ALBERT"},
		{"Anne", "ZOLA"},
		{"Bruno", "Étienne"},
		{"Claire", "DURAND"},
	}
	sort.SliceStable(membres, func(i, j int) bool {
		return memberSortKey(membres[i].prenom, membres[i].nom) <
			memberSortKey(membres[j].prenom, membres[j].nom)
	})

	attendu := []string{"ALBERT", "DURAND", "Étienne", "ZOLA"}
	for i, want := range attendu {
		if membres[i].nom != want {
			t.Fatalf("position %d : got %q, want %q (ordre obtenu : %v)", i, membres[i].nom, want, membres)
		}
	}
}

// Deux homonymes se départagent sur le prénom.
func TestMemberSortKeyBreaksTiesOnFirstName(t *testing.T) {
	if memberSortKey("Bernard", "MARTIN") >= memberSortKey("Yves", "MARTIN") {
		t.Fatal("MARTIN Bernard doit précéder MARTIN Yves")
	}
}

// Un nom court suivi d'un prénom ne doit pas déborder sur le nom suivant.
func TestMemberSortKeyDoesNotBleedIntoFirstName(t *testing.T) {
	if memberSortKey("André", "DUR") >= memberSortKey("Claire", "DURAND") {
		t.Fatal("DUR André doit précéder DURAND Claire")
	}
}

func TestFoldForSortStripsCaseAndAccents(t *testing.T) {
	cases := map[string]string{
		"Étienne":   "etienne",
		"  LEBŒUF ": "leboeuf",
		"Nuñez":     "nunez",
		"Çağla":     "cağla",
	}
	for in, want := range cases {
		if got := foldForSort(in); got != want {
			t.Errorf("foldForSort(%q) = %q, want %q", in, got, want)
		}
	}
}
