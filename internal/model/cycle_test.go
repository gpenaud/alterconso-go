package model

import (
	"testing"
	"time"
)

// Les rythmes longs se comptent en mois, pas en jours : douze fois trente
// jours font onze mois et demi, et un cycle annuel compté ainsi décalerait
// d'une semaine à chaque tour.
func TestLongRhythmsDoNotDrift(t *testing.T) {
	depart := time.Date(2027, 3, 4, 18, 0, 0, 0, time.UTC)

	annuel := DistributionCycle{IntervalMonths: 12}
	if got := annuel.Next(depart); !got.Equal(time.Date(2028, 3, 4, 18, 0, 0, 0, time.UTC)) {
		t.Errorf("un an après le 04/03/2027, obtenu %s", got.Format("02/01/2006"))
	}

	semestriel := DistributionCycle{IntervalMonths: 6}
	if got := semestriel.Next(depart); !got.Equal(time.Date(2027, 9, 4, 18, 0, 0, 0, time.UTC)) {
		t.Errorf("six mois après le 04/03/2027, obtenu %s", got.Format("02/01/2006"))
	}

	// Sur quatre tours, un cycle semestriel doit retomber deux ans plus tard,
	// au même jour. C'est ce qu'un décompte en jours perdrait.
	d := depart
	for i := 0; i < 4; i++ {
		d = semestriel.Next(d)
	}
	if !d.Equal(time.Date(2029, 3, 4, 18, 0, 0, 0, time.UTC)) {
		t.Errorf("quatre semestres plus tard, obtenu %s au lieu du 04/03/2029", d.Format("02/01/2006"))
	}
}

// Les rythmes courts restent en jours : ils doivent tomber le même jour de la
// semaine, ce qu'on attend d'une distribution hebdomadaire.
func TestShortRhythmsKeepTheWeekday(t *testing.T) {
	jeudi := time.Date(2027, 3, 4, 18, 0, 0, 0, time.UTC)
	for _, days := range []int{7, 14, 21} {
		c := DistributionCycle{IntervalDays: days}
		if got := c.Next(jeudi); got.Weekday() != jeudi.Weekday() {
			t.Errorf("rythme de %d jours : passé de %s à %s", days, jeudi.Weekday(), got.Weekday())
		}
	}
}

// Le mois l'emporte quand les deux sont posés : c'est l'unité la plus précise
// pour un rythme long.
func TestMonthsWinOverDays(t *testing.T) {
	c := DistributionCycle{IntervalDays: 7, IntervalMonths: 6}
	depart := time.Date(2027, 3, 4, 0, 0, 0, 0, time.UTC)
	if got := c.Next(depart); got.Month() != time.September {
		t.Errorf("le rythme mensuel devrait primer, obtenu %s", got.Format("02/01/2006"))
	}
}

// La périodicité se dit en toutes lettres, quelle que soit son unité.
func TestRhythmIsSpelledOut(t *testing.T) {
	cases := []struct {
		c    DistributionCycle
		want string
	}{
		{DistributionCycle{IntervalDays: 7}, "Toutes les semaines"},
		{DistributionCycle{IntervalDays: 14}, "Tous les quinze jours"},
		{DistributionCycle{IntervalDays: 21}, "Toutes les trois semaines"},
		{DistributionCycle{IntervalMonths: 1}, "Tous les mois"},
		{DistributionCycle{IntervalMonths: 6}, "Tous les six mois"},
		{DistributionCycle{IntervalMonths: 12}, "Tous les ans"},
		// Les cycles créés avant l'unité « mois » gardent leur libellé.
		{DistributionCycle{IntervalDays: 30}, "Tous les mois"},
	}
	for _, tc := range cases {
		if got := tc.c.RhythmLabel(); got != tc.want {
			t.Errorf("obtenu %q, attendu %q", got, tc.want)
		}
	}
}
