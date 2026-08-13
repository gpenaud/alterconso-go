package handler

import (
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

func TestOrderQtyLabel(t *testing.T) {
	cases := []struct {
		name     string
		quantity float64
		qt       *float64
		unit     model.UnitType
		want     string
	}{
		{"parts d'un produit conditionne", 3, ptrFloat(500), model.UnitTypeGram, "3 × 500 g"},
		{"le kilo reste un kilo, pas trois", 3, ptrFloat(1), model.UnitTypeKilogram, "3 × 1 kg"},
		{"conditionnement fractionnaire", 2, ptrFloat(0.5), model.UnitTypeKilogram, "2 × 1/2 kg"},
		{"bouteille en centilitres", 1, ptrFloat(75), model.UnitTypeCentilitre, "1 × 75 cl"},
		{"piece unitaire, rien a multiplier", 3, ptrFloat(1), model.UnitTypePiece, "3 pièces"},
		{"une seule piece reste au singulier", 1, ptrFloat(1), model.UnitTypePiece, "1 pièce"},
		{"lot de pieces", 2, ptrFloat(6), model.UnitTypePiece, "2 × 6 pièces"},
		{"quantite produit absente vaut une unite", 4, nil, model.UnitTypePiece, "4 pièces"},
		{"unite legacy vide traitee comme la piece", 2, nil, "", "2 pièces"},
		{"demi-part apres pesee", 0.5, ptrFloat(1), model.UnitTypeKilogram, "0.5 × 1 kg"},
		{"total agrege reste decimal, pas une fraction", 2.5, ptrFloat(1), model.UnitTypeKilogram, "2.5 × 1 kg"},
		{"conditionnement en demi-part garde la fraction", 3, ptrFloat(0.5), model.UnitTypeLitre, "3 × 1/2 L"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := model.Product{Qt: tc.qt, UnitType: tc.unit}
			if got := orderQtyLabel(tc.quantity, p); got != tc.want {
				t.Fatalf("attendu %q, obtenu %q", tc.want, got)
			}
		})
	}
}
