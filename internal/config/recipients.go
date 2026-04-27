package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RecipientCategory définit une catégorie de destinataires de messages basée
// sur l'activité (nombre de commandes sur une fenêtre temporelle).
//
// Champs YAML : name, pattern.
// Format de pattern : "<op> <N> commande[s] / <T>m"
//   op : <, <=, >, >=, =
//   N  : nombre entier de commandes (distributions distinctes)
//   T  : nombre de mois sur lequel on regarde en arrière
//
// Exemples :
//   ">= 3 commandes / 3m" → membres réguliers
//   ">= 1 commande / 6m"  → membres actifs
//   "< 1 commande / 6m"   → membres inactifs
//
// Les catégories sont mutuellement exclusives : un utilisateur appartient à
// la première catégorie qui matche, dans l'ordre du fichier YAML.
type RecipientCategory struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`

	// Champs dérivés du pattern (remplis par Validate()).
	Op        string        `yaml:"-"`
	Threshold int           `yaml:"-"`
	Window    time.Duration `yaml:"-"`
}

// MessagesConfig regroupe la configuration de la page messages.
type MessagesConfig struct {
	RecipientCategories []RecipientCategory `yaml:"recipient_categories"`
}

var recipientPatternRe = regexp.MustCompile(`^\s*(<=|>=|<|>|=)\s*(\d+)\s+commandes?\s*/\s*(\d+)\s*m\s*$`)

// ParseRecipientPattern parse une expression de catégorie.
// Retourne op, threshold, window.
func ParseRecipientPattern(pattern string) (string, int, time.Duration, error) {
	m := recipientPatternRe.FindStringSubmatch(strings.TrimSpace(pattern))
	if m == nil {
		return "", 0, 0, fmt.Errorf("pattern invalide %q (attendu : \"<op> N commande[s] / Tm\")", pattern)
	}
	threshold, _ := strconv.Atoi(m[2])
	months, _ := strconv.Atoi(m[3])
	if months <= 0 {
		return "", 0, 0, fmt.Errorf("pattern %q : la fenêtre doit être > 0 mois", pattern)
	}
	return m[1], threshold, time.Duration(months) * 30 * 24 * time.Hour, nil
}

// Compact retourne la règle sous forme courte pour l'UI, ex :
//   ">= 3 commandes / 3m"  →  "≥3 commandes / 3 mois"
//   "< 1 commande / 6m"    →  "<1 commande / 6 mois"
func (cat *RecipientCategory) Compact() string {
	months := int(cat.Window / (30 * 24 * time.Hour))
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
	return fmt.Sprintf("%s%d %s / %d mois", op, cat.Threshold, noun, months)
}

// Match évalue si un nombre de commandes satisfait la règle de la catégorie.
func (cat *RecipientCategory) Match(orderCount int) bool {
	switch cat.Op {
	case "<":
		return orderCount < cat.Threshold
	case "<=":
		return orderCount <= cat.Threshold
	case ">":
		return orderCount > cat.Threshold
	case ">=":
		return orderCount >= cat.Threshold
	case "=":
		return orderCount == cat.Threshold
	}
	return false
}
