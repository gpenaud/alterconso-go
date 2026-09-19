package handler

import "testing"

func TestSommeSaisie(t *testing.T) {
	cas := []struct {
		brut   string
		valeur float64
		ok     bool
	}{
		{"12.50", 12.50, true},
		{"12,50", 12.50, true},          // la virgule francaise
		{"4,20+3,15+3,85", 11.20, true}, // le cas du producteur
		{" 4,20 + 3,15 ", 7.35, true},   // les espaces de frappe
		{"10-2,5", 7.5, true},           // une reprise en moins
		{"+5", 5, true},
		{"5+", 0, false}, // terme manquant
		{"4,20++3", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"4*3", 0, false},   // ni multiplication
		{"(4+3)", 0, false}, // ni parentheses
		{"4,20+3,15+3,85+2,00", 13.20, true},
	}
	for _, c := range cas {
		v, ok := sommeSaisie(c.brut)
		if ok != c.ok {
			t.Errorf("%q : accepte=%v, attendu %v", c.brut, ok, c.ok)
			continue
		}
		if ok && (v-c.valeur > 0.0001 || c.valeur-v > 0.0001) {
			t.Errorf("%q : %v, attendu %v", c.brut, v, c.valeur)
		}
	}
}
