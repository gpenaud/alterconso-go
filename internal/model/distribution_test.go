package model

import (
	"testing"
	"time"
)

// orderable : un catalogue dont les membres peuvent commander en ligne.
func orderable() Catalog {
	c := Catalog{}
	c.Flags = uint(CatalogFlagUsersCanOrder)
	return c
}

func at(d time.Duration) *time.Time {
	t := time.Now().Add(d)
	return &t
}

func TestDistributionCanOrderNow(t *testing.T) {
	const jour = 24 * time.Hour

	cases := []struct {
		nom  string
		d    Distribution
		want bool
	}{
		{
			"catalogue fermé aux commandes en ligne",
			Distribution{Catalog: Catalog{}},
			false,
		},
		{
			"aucune clôture : rien ne borne les commandes",
			Distribution{Catalog: orderable()},
			true,
		},
		{
			"clôture du jour à venir",
			Distribution{Catalog: orderable(), MultiDistrib: MultiDistrib{
				OrderStartDate: at(-2 * jour), OrderEndDate: at(jour)}},
			true,
		},
		{
			"clôture du jour dépassée",
			Distribution{Catalog: orderable(), MultiDistrib: MultiDistrib{
				OrderStartDate: at(-3 * jour), OrderEndDate: at(-jour)}},
			false,
		},
		{
			"ouverture encore à venir",
			Distribution{Catalog: orderable(), MultiDistrib: MultiDistrib{
				OrderStartDate: at(jour), OrderEndDate: at(2 * jour)}},
			false,
		},
		{
			// Le cas que produit la réouverture depuis la fiche catalogue : la
			// distribution surcharge sa seule date de fin. Lire les deux dates
			// d'un bloc laissait ici une date de début nulle, déréférencée
			// aussitôt — panic, et non « false ».
			"réouverture : clôture propre à la distribution, sans ouverture",
			Distribution{Catalog: orderable(), OrderEndDate: at(jour)},
			true,
		},
		{
			"réouverture par-dessus une clôture du jour dépassée",
			Distribution{
				Catalog:      orderable(),
				OrderEndDate: at(jour),
				MultiDistrib: MultiDistrib{OrderStartDate: at(-3 * jour), OrderEndDate: at(-jour)},
			},
			true,
		},
		{
			// La surcharge vaut dans les deux sens : elle sert aussi à fermer
			// plus tôt que le jour ne le prévoit.
			"fermeture anticipée pour ce seul catalogue",
			Distribution{
				Catalog:      orderable(),
				OrderEndDate: at(-time.Hour),
				MultiDistrib: MultiDistrib{OrderStartDate: at(-3 * jour), OrderEndDate: at(jour)},
			},
			false,
		},
		{
			"la surcharge garde l'ouverture du jour, encore à venir",
			Distribution{
				Catalog:      orderable(),
				OrderEndDate: at(2 * jour),
				MultiDistrib: MultiDistrib{OrderStartDate: at(jour)},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			d := tc.d
			if got := d.CanOrderNow(); got != tc.want {
				t.Errorf("CanOrderNow = %v, attendu %v", got, tc.want)
			}
		})
	}
}

// Le scénario visé : le jour est clos, un seul producteur a repoussé sa
// clôture. Lui seul redevient commandable ; ses voisins restent fermés.
func TestReopeningIsPerCatalogue(t *testing.T) {
	jour := MultiDistrib{
		DistribStartDate: time.Now().Add(48 * time.Hour),
		OrderStartDate:   at(-5 * 24 * time.Hour),
		OrderEndDate:     at(-2 * time.Hour), // clôture commune dépassée
	}

	rouvert := Distribution{Catalog: orderable(), MultiDistrib: jour, OrderEndDate: at(12 * time.Hour)}
	voisin := Distribution{Catalog: orderable(), MultiDistrib: jour}

	if !rouvert.CanOrderNow() {
		t.Error("le catalogue rouvert doit accepter des commandes")
	}
	if voisin.CanOrderNow() {
		t.Error("le catalogue voisin doit rester fermé")
	}
	// La réouverture ne dépasse pas la veille de la livraison.
	if end := rouvert.EffectiveOrderEnd(); end.After(rouvert.MaxOrderEnd()) {
		t.Errorf("clôture %v au-delà de la limite %v", end, rouvert.MaxOrderEnd())
	}
}

// Ce qu'un gestionnaire peut reprendre : une période commencée, close ou non.
// Jamais une période à venir — il n'y aurait rien à y corriger, seulement des
// commandes à passer avant les membres.
func TestOrderWindowStarted(t *testing.T) {
	now := time.Now()
	const jour = 24 * time.Hour

	cases := []struct {
		nom       string
		d         Distribution
		commencée bool
	}{
		{
			"période en cours",
			Distribution{MultiDistrib: MultiDistrib{OrderStartDate: at(-jour), OrderEndDate: at(jour)}},
			true,
		},
		{
			"période close — le cas de la correction après coup",
			Distribution{MultiDistrib: MultiDistrib{OrderStartDate: at(-3 * jour), OrderEndDate: at(-jour)}},
			true,
		},
		{
			"période encore à venir",
			Distribution{MultiDistrib: MultiDistrib{OrderStartDate: at(jour), OrderEndDate: at(2 * jour)}},
			false,
		},
		{
			"ouverture propre au catalogue, à venir",
			Distribution{
				OrderStartDate: at(jour),
				MultiDistrib:   MultiDistrib{OrderStartDate: at(-3 * jour), OrderEndDate: at(-jour)},
			},
			false,
		},
		{
			"aucune ouverture : rien n'attend",
			Distribution{},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			d := tc.d
			if got := d.OrderWindowStarted(now); got != tc.commencée {
				t.Errorf("OrderWindowStarted = %v, attendu %v", got, tc.commencée)
			}
		})
	}
}

// Une ouverture se fixe à partir du jour en cours, jamais dans le passé.
func TestMinOrderStart(t *testing.T) {
	now := time.Date(2026, 8, 14, 15, 30, 0, 0, time.Local)
	floor := MinOrderStart(now)

	want := time.Date(2026, 8, 14, 0, 0, 0, 0, time.Local)
	if !floor.Equal(want) {
		t.Fatalf("plancher attendu %v, obtenu %v", want, floor)
	}

	cases := []struct {
		nom     string
		saisie  time.Time
		refusée bool
	}{
		{"hier", now.AddDate(0, 0, -1), true},
		{"la veille au soir", time.Date(2026, 8, 13, 23, 59, 0, 0, time.Local), true},
		// Le plancher tombe à minuit, et non à l'heure courante : régler
		// l'après-midi une ouverture datée du matin même reste possible.
		{"ce matin", time.Date(2026, 8, 14, 8, 0, 0, 0, time.Local), false},
		{"tout à l'heure", now.Add(2 * time.Hour), false},
		{"demain", now.AddDate(0, 0, 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := tc.saisie.Before(floor); got != tc.refusée {
				t.Errorf("refus = %v, attendu %v", got, tc.refusée)
			}
		})
	}
}

// La clôture qu'un producteur peut se donner s'arrête à la veille de la
// livraison : repousser sert à prendre une commande tardive, pas à la prendre
// le matin même.
func TestMaxOrderEnd(t *testing.T) {
	jour := time.Date(2026, 8, 20, 17, 0, 0, 0, time.Local)

	d := Distribution{MultiDistrib: MultiDistrib{DistribStartDate: jour}}
	want := time.Date(2026, 8, 19, 17, 0, 0, 0, time.Local)
	if got := d.MaxOrderEnd(); !got.Equal(want) {
		t.Errorf("limite attendue %v, obtenue %v", want, got)
	}

	// La limite suit la livraison quand celle-ci est propre au catalogue.
	propre := time.Date(2026, 8, 22, 10, 0, 0, 0, time.Local)
	d.Date = &propre
	want = time.Date(2026, 8, 21, 10, 0, 0, 0, time.Local)
	if got := d.MaxOrderEnd(); !got.Equal(want) {
		t.Errorf("limite attendue %v, obtenue %v", want, got)
	}
}

func TestDistributionEffectiveOrderDates(t *testing.T) {
	propre, jour := at(time.Hour), at(48*time.Hour)

	d := Distribution{OrderEndDate: propre, MultiDistrib: MultiDistrib{OrderEndDate: jour}}
	if d.EffectiveOrderEnd() != propre {
		t.Error("la clôture de la distribution doit primer sur celle du jour")
	}

	héritée := Distribution{MultiDistrib: MultiDistrib{OrderEndDate: jour}}
	if héritée.EffectiveOrderEnd() != jour {
		t.Error("sans surcharge, la clôture du jour s'applique")
	}

	aucune := Distribution{}
	if aucune.EffectiveOrderEnd() != nil || aucune.EffectiveOrderStart() != nil {
		t.Error("sans aucune date, rien ne borne les commandes")
	}
}

// Ce qui fait qu'une date « déroge » : qu'elle diffère du jour, non qu'elle
// existe. Presque toutes les distributions portent une copie des dates du
// jour ; les marquer toutes comme exceptions revenait à n'en marquer aucune.
func TestDerogations(t *testing.T) {
	jour := time.Date(2026, 9, 4, 17, 0, 0, 0, time.Local)
	ouverture := time.Date(2026, 8, 28, 8, 0, 0, 0, time.Local)
	cloture := time.Date(2026, 9, 3, 9, 0, 0, 0, time.Local)
	autre := time.Date(2026, 8, 20, 9, 0, 0, 0, time.Local)

	md := MultiDistrib{DistribStartDate: jour, OrderStartDate: &ouverture, OrderEndDate: &cloture}
	copie := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		nom              string
		d                Distribution
		date, start, end bool
	}{
		{"aucune valeur propre", Distribution{MultiDistrib: md}, false, false, false},
		{
			// Le cas de la quasi-totalité des distributions en base.
			"copie conforme du jour",
			Distribution{MultiDistrib: md, Date: copie(jour), OrderStartDate: copie(ouverture), OrderEndDate: copie(cloture)},
			false, false, false,
		},
		{
			"clôture avancée",
			Distribution{MultiDistrib: md, Date: copie(jour), OrderEndDate: copie(autre)},
			false, false, true,
		},
		{
			"livraison décalée d'une heure",
			Distribution{MultiDistrib: md, Date: copie(jour.Add(time.Hour))},
			true, false, false,
		},
		{
			"borne posée là où le jour n'en a pas",
			Distribution{MultiDistrib: MultiDistrib{DistribStartDate: jour}, OrderEndDate: copie(cloture)},
			false, false, true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			d := tc.d
			if got := d.DateDerogates(); got != tc.date {
				t.Errorf("DateDerogates = %v, attendu %v", got, tc.date)
			}
			if got := d.OrderStartDerogates(); got != tc.start {
				t.Errorf("OrderStartDerogates = %v, attendu %v", got, tc.start)
			}
			if got := d.OrderEndDerogates(); got != tc.end {
				t.Errorf("OrderEndDerogates = %v, attendu %v", got, tc.end)
			}
		})
	}
}
