package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RecipientCategory définit une catégorie de destinataires basée sur l'activité.
//
// Deux formats de pattern sont supportés :
//
//  1. Total sur la fenêtre — somme des commandes sur les T derniers mois :
//     "<op> N commande[s] / Tm"
//     ex : ">= 3 commandes / 3m" → ≥3 commandes au total sur les 3 derniers mois.
//
//  2. Par mois — règle appliquée individuellement à chaque mois calendaire de
//     la fenêtre, le user matche s'il satisfait la règle dans CHAQUE mois :
//     "<op> N commande[s]/mois sur Tm"
//     ex : ">= 1 commande/mois sur 6m" → ≥1 commande dans chacun des 6 derniers
//     mois calendaires (mois courant inclus).
//
// op ∈ {<, <=, >, >=, =} — appliqué au total (mode 1) ou au compteur d'un mois
// donné (mode 2).
//
// Les catégories sont mutuellement exclusives sur la page /messages : un user
// est attribué à la PREMIÈRE catégorie qui matche, dans l'ordre du fichier.
type RecipientCategory struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`

	// Champs dérivés du pattern (remplis au load).
	Op        string        `yaml:"-"`
	Threshold int           `yaml:"-"`
	Window    time.Duration `yaml:"-"`
	PerMonth  bool          `yaml:"-"` // true si pattern de la forme "/mois sur Tm"
}

// MessagesConfig regroupe la configuration de la page messages.
type MessagesConfig struct {
	RecipientCategories []RecipientCategory `yaml:"recipient_categories"`
}

var (
	totalPatternRe    = regexp.MustCompile(`^\s*(<=|>=|<|>|=)\s*(\d+)\s+commandes?\s*/\s*(\d+)\s*m\s*$`)
	perMonthPatternRe = regexp.MustCompile(`^\s*(<=|>=|<|>|=)\s*(\d+)\s+commandes?\s*/\s*mois\s+sur\s+(\d+)\s*m\s*$`)
)

// ParseRecipientPattern parse une expression de catégorie.
// Retourne (op, threshold, window, perMonth, err).
func ParseRecipientPattern(pattern string) (string, int, time.Duration, bool, error) {
	s := strings.TrimSpace(pattern)
	if m := perMonthPatternRe.FindStringSubmatch(s); m != nil {
		threshold, _ := strconv.Atoi(m[2])
		months, _ := strconv.Atoi(m[3])
		if months <= 0 {
			return "", 0, 0, false, fmt.Errorf("pattern %q : la fenêtre doit être > 0 mois", pattern)
		}
		return m[1], threshold, time.Duration(months) * 30 * 24 * time.Hour, true, nil
	}
	if m := totalPatternRe.FindStringSubmatch(s); m != nil {
		threshold, _ := strconv.Atoi(m[2])
		months, _ := strconv.Atoi(m[3])
		if months <= 0 {
			return "", 0, 0, false, fmt.Errorf("pattern %q : la fenêtre doit être > 0 mois", pattern)
		}
		return m[1], threshold, time.Duration(months) * 30 * 24 * time.Hour, false, nil
	}
	return "", 0, 0, false, fmt.Errorf(
		"pattern invalide %q (attendu : %q ou %q)",
		pattern,
		"<op> N commande[s] / Tm",
		"<op> N commande[s]/mois sur Tm",
	)
}

// WindowMonths retourne la fenêtre en mois (approximée à 30 jours/mois).
func (cat *RecipientCategory) WindowMonths() int {
	return int(cat.Window / (30 * 24 * time.Hour))
}

// compareCount applique cat.Op pour comparer un compteur au seuil.
func (cat *RecipientCategory) compareCount(value int) bool {
	switch cat.Op {
	case "<":
		return value < cat.Threshold
	case "<=":
		return value <= cat.Threshold
	case ">":
		return value > cat.Threshold
	case ">=":
		return value >= cat.Threshold
	case "=":
		return value == cat.Threshold
	}
	return false
}

// Match évalue si le score d'un user satisfait la règle.
//
// Le « score » varie selon le mode :
//   - Mode total : score = nombre total de commandes dans la fenêtre.
//     Match = `score op Threshold`.
//   - Mode per-month : score = nombre de mois calendaires de la fenêtre
//     dans lesquels le user satisfait la règle (cf. MatchPerMonthCount).
//     Match = true si tous les mois de la fenêtre qualifient
//     (score >= WindowMonths).
func (cat *RecipientCategory) Match(score int) bool {
	if cat.PerMonth {
		return score >= cat.WindowMonths()
	}
	return cat.compareCount(score)
}

// MatchPerMonthCount applique la règle au compteur d'un mois individuel.
// Utilisé en mode per-month pour décider si un mois donné qualifie.
func (cat *RecipientCategory) MatchPerMonthCount(monthCount int) bool {
	return cat.compareCount(monthCount)
}

// Compact retourne la règle sous forme courte pour l'UI :
//   ">= 3 commandes / 3m"        → "≥3 commandes / 3 mois"
//   "< 1 commande / 6m"          → "<1 commande / 6 mois"
//   ">= 1 commande/mois sur 6m"  → "≥1 commande/mois sur 6 mois"
func (cat *RecipientCategory) Compact() string {
	months := cat.WindowMonths()
	noun := "commande"
	if cat.Threshold > 1 {
		noun = "commandes"
	}
	op := cat.Op
	switch op {
	case "<=":
		op = "≤"
	case ">=":
		op = "≥"
	}
	if cat.PerMonth {
		return fmt.Sprintf("%s%d %s/mois sur %d mois", op, cat.Threshold, noun, months)
	}
	return fmt.Sprintf("%s%d %s / %d mois", op, cat.Threshold, noun, months)
}
