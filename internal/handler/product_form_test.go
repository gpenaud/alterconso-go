package handler

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
)

// postForm construit un contexte Gin portant le formulaire donné.
func postForm(values url.Values) *gin.Context {
	req := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

// La sous-catégorie « Tous » de chaque catégorie, telle que txpCategories la
// construirait depuis la base.
var subByCategory = map[uint]uint{3: 30, 5: 50}

func TestApplyProductFormStock(t *testing.T) {
	cases := []struct {
		name  string
		saisi string
		want  *float64
	}{
		{"entier", "12", ptrFloat(12)},
		{"zero, soit une rupture", "0", ptrFloat(0)},
		{"vide, soit un stock non suivi", "", nil},
		{"decimal tronque plutot qu'ignore", "3,7", ptrFloat(3)},
		{"negatif refuse", "-4", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := model.Product{}
			applyProductForm(postForm(url.Values{"name": {"Miel"}, "stock": {tc.saisi}}), &p, subByCategory)

			switch {
			case tc.want == nil && p.Stock != nil:
				t.Fatalf("stock attendu nul, obtenu %v", *p.Stock)
			case tc.want != nil && p.Stock == nil:
				t.Fatalf("stock attendu %v, obtenu nul", *tc.want)
			case tc.want != nil && *p.Stock != *tc.want:
				t.Fatalf("stock attendu %v, obtenu %v", *tc.want, *p.Stock)
			}
		})
	}
}

func TestApplyProductFormCategory(t *testing.T) {
	// Une catégorie choisie se traduit par sa sous-catégorie « Tous ».
	p := model.Product{}
	applyProductForm(postForm(url.Values{"name": {"Miel"}, "txp_category": {"5"}}), &p, subByCategory)
	if p.TxpSubCategoryID == nil || *p.TxpSubCategoryID != 50 {
		t.Fatalf("sous-catégorie attendue 50, obtenue %v", p.TxpSubCategoryID)
	}

	// « Autres » déclasse le produit, au lieu de conserver l'ancienne valeur.
	applyProductForm(postForm(url.Values{"name": {"Miel"}, "txp_category": {""}}), &p, subByCategory)
	if p.TxpSubCategoryID != nil {
		t.Fatalf("sous-catégorie attendue nulle, obtenue %v", *p.TxpSubCategoryID)
	}
}

// TestProductColumnsWritesStockAndCategory garde la trace du défaut corrigé :
// les deux colonnes manquaient à la map, donc l'édition ne les écrivait jamais.
func TestProductColumnsWritesStockAndCategory(t *testing.T) {
	for _, col := range []string{"stock", "txp_sub_category_id"} {
		if _, ok := productColumns(&model.Product{})[col]; !ok {
			t.Errorf("colonne %q absente : elle ne sera pas enregistrée à l'édition", col)
		}
	}
}

func ptrFloat(v float64) *float64 { return &v }
