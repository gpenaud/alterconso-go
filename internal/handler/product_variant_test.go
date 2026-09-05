package handler

import "testing"

// Deux calibres d'un même produit portent la même photographie : dans un
// aperçu de huit vignettes, en montrer deux gâche une place.
func TestVariantsOfTheSameProductShareAKey(t *testing.T) {
	memes := [][2]string{
		{"POULET BIO 1.8 à 1.9kg", "POULET BIO 1.5 à 1.6kg"},
		{"Fondue Bressane - Sous vide 800g", "Fondue Bressane - Sous vide 400g"},
		{"Tomme de chèvre 250g", "Tomme de chèvre 500 g"},
		{"Jus de pomme 1L", "Jus de pomme 25cl"},
		{"Œufs x6", "Œufs x12"},
	}
	for _, m := range memes {
		if a, b := productVariantKey(m[0]), productVariantKey(m[1]); a != b {
			t.Errorf("%q et %q devraient se confondre, obtenu %q ≠ %q", m[0], m[1], a, b)
		}
	}

	// Et deux produits différents restent distincts.
	differents := [][2]string{
		{"POULET BIO 1.8 à 1.9kg", "Pintade fermière 1.4kg"},
		{"Tomme de chèvre", "Bûche cendrée de chèvre"},
		{"Jus de pomme", "Jus de caseille"},
	}
	for _, d := range differents {
		if a, b := productVariantKey(d[0]), productVariantKey(d[1]); a == b {
			t.Errorf("%q et %q ne devraient pas se confondre (clé %q)", d[0], d[1], a)
		}
	}
}

// La bande ne garde qu'un conditionnement, en conservant l'ordre.
func TestDedupeKeepsFirstVariant(t *testing.T) {
	out := dedupeVariants([]ProductImageView{
		{Name: "POULET BIO 1.8 à 1.9kg"},
		{Name: "Bûche cendrée de chèvre"},
		{Name: "POULET BIO 1.5 à 1.6kg"},
		{Name: "POULET BIO 2.0 à 2.2kg"},
	})
	if len(out) != 2 {
		t.Fatalf("deux produits distincts attendus, obtenu %d : %v", len(out), out)
	}
	if out[0].Name != "POULET BIO 1.8 à 1.9kg" || out[1].Name != "Bûche cendrée de chèvre" {
		t.Errorf("le premier conditionnement rencontré doit rester, obtenu %v", out)
	}
}

// Un nom qui se réduit à rien ne doit pas confondre tous ses semblables.
func TestEmptyKeysDoNotCollapse(t *testing.T) {
	out := dedupeVariants([]ProductImageView{{Name: "500 g"}, {Name: "1 kg"}, {Name: "12"}})
	if len(out) != 3 {
		t.Errorf("aucun ne doit disparaître faute de nom exploitable, obtenu %d", len(out))
	}
}

// Deux parfums d'une même tisane se ressemblent trop pour mériter deux places
// dans un aperçu de huit vignettes.
func TestFlavourVariantsShareAFamily(t *testing.T) {
	memes := [][2]string{
		{"RILLETTE DE POULET BIO AU THYM EN BOCAUX", "RILLETTE DE POULET BIO NATURE EN BOCAUX"},
		{"Tisane paysanne - Éveil de printemps", "Tisane paysanne - Évasion nocturne"},
		{"Confiture de fraise", "Confiture de framboise"},
	}
	for _, m := range memes {
		if a, b := productFamilyKey(m[0]), productFamilyKey(m[1]); a != b {
			t.Errorf("%q et %q devraient former une famille, obtenu %q ≠ %q", m[0], m[1], a, b)
		}
	}
}

// Mais des produits courts et distincts ne se confondent pas : les rapprocher
// appauvrirait l'aperçu au lieu de le varier.
func TestShortNamesStayDistinct(t *testing.T) {
	distincts := [][2]string{
		{"Jus de Caseille", "Jus de pomme"},
		{"Bûche cendrée de chèvre", "Tomme de chèvre"},
		{"Farine de Sarrasin", "Farine de Blé"},
	}
	for _, d := range distincts {
		if a, b := productFamilyKey(d[0]), productFamilyKey(d[1]); a == b {
			t.Errorf("%q et %q ne devraient pas se confondre (clé %q)", d[0], d[1], a)
		}
	}
}

// La bande arbitre, elle ne masque rien : le volet du producteur garde tout.
func TestFamilyDedupeKeepsFirstAndPreservesOrder(t *testing.T) {
	out := dedupeFamilies([]ProductImageView{
		{Name: "RILLETTE DE POULET BIO AU THYM EN BOCAUX"},
		{Name: "Jus de Caseille"},
		{Name: "RILLETTE DE POULET BIO NATURE EN BOCAUX"},
	})
	if len(out) != 2 {
		t.Fatalf("deux familles attendues, obtenu %d : %v", len(out), out)
	}
	if out[0].Name != "RILLETTE DE POULET BIO AU THYM EN BOCAUX" || out[1].Name != "Jus de Caseille" {
		t.Errorf("l'ordre et le premier venu doivent tenir, obtenu %v", out)
	}
}
