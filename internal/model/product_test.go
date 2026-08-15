package model

import "testing"

func TestProductStockLimit(t *testing.T) {
	qty := func(v float64) *float64 { return &v }

	cases := []struct {
		nom     string
		p       Product
		limite  float64
		bornant bool
	}{
		{
			"suivi actif, quantité saisie",
			Product{StockTracked: true, Stock: qty(21)},
			21, true,
		},
		{
			// Zéro est une valeur : le produit est épuisé, et le dépassement
			// doit se signaler dès le premier article commandé.
			"suivi actif, stock épuisé",
			Product{StockTracked: true, Stock: qty(0)},
			0, true,
		},
		{
			// Le cas observé sur « Les Oliviers de Marianne » : la case est
			// cochée, la quantité jamais saisie. Lu comme un zéro, il faisait
			// passer chaque commande pour un dépassement.
			"suivi actif, quantité jamais renseignée",
			Product{StockTracked: true, Stock: nil},
			0, false,
		},
		{
			"suivi inactif, quantité résiduelle en base",
			Product{StockTracked: false, Stock: qty(12)},
			0, false,
		},
		{
			"ni suivi ni quantité",
			Product{},
			0, false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			p := tc.p
			limite, bornant := p.StockLimit()
			if bornant != tc.bornant {
				t.Fatalf("bornant = %v, attendu %v", bornant, tc.bornant)
			}
			if bornant && limite != tc.limite {
				t.Errorf("limite = %v, attendue %v", limite, tc.limite)
			}
		})
	}
}
