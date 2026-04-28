package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RecipientCategory définit une catégorie de destinataires sur la page /messages.
//
// Une catégorie est composée d'une ou plusieurs conditions, combinées par AND ou OR.
// Trois formes de saisie YAML, équivalentes pour une condition unique :
//
//   - Une seule condition, forme courte :
//       pattern: ">= 3 commandes / 3m"
//
//   - Plusieurs conditions liées par ET (le user doit toutes les satisfaire) :
//       all:
//         - ">= 3 commandes / 3m"
//         - ">= 1 commande/mois sur 1m"
//
//   - Plusieurs conditions liées par OU (au moins une suffit) :
//       any:
//         - "is guillaume.penaud@gmail.com"
//         - ">= 5 commandes / 1m"
//
// Une seule de ces trois clés (`pattern`, `all`, `any`) doit être renseignée par catégorie.
//
// Trois formats de pattern individuel :
//
//  1. Total fenêtre — "<op> N commande[s] sur Tm"
//     ex : ">= 3 commandes sur 3m" → ≥3 sur les 3 derniers mois (fenêtre glissante).
//     La forme historique avec "/" reste acceptée par rétrocompat.
//
//  2. Par mois calendaire — "<op> N commande[s]/mois sur Tm"
//     ex : ">= 1 commande/mois sur 6m" → ≥1 dans CHACUN des 6 derniers mois.
//
//  3. Email ciblé — "is <email>"
//     ex : "is admin@exemple.fr" → ce user uniquement.
//
// Sémantique des ensembles :
//
// Par défaut, les catégories sont mutuellement exclusives : un user appartient
// à la PREMIÈRE catégorie qui matche, dans l'ordre du fichier (sa "primary").
// Toute catégorie ultérieure dont il matche aussi le pattern l'IGNORE par défaut.
//
// La directive `includes` permet à une catégorie d'agréger explicitement les
// "primary" d'autres catégories. L'ensemble final est :
//
//   set(C) = primary(C) ∪ ⋃ primary(I) pour I ∈ C.Includes
//
// L'inclusion n'est pas récursive : seuls les primaires des catégories listées
// sont ajoutés. Pour propager, lister explicitement.
//
// Conséquence : les catégories peuvent se chevaucher (un user peut appartenir
// à plusieurs catégories via `includes`). La déduplication à l'envoi est
// implicite (un user reçoit le mail une fois quelle que soit la catégorie
// sélectionnée).
type RecipientCategory struct {
	Name     string   `yaml:"name"`
	Pattern  string   `yaml:"pattern"`
	All      []string `yaml:"all"`
	Any      []string `yaml:"any"`
	Includes []string `yaml:"includes"`

	// Compilé au load.
	Conditions []ParsedRecipientPattern `yaml:"-"`
	Combinator string                   `yaml:"-"` // "all" (AND) ou "any" (OR)
}

// MessagesConfig regroupe la configuration de la page messages.
type MessagesConfig struct {
	RecipientCategories []RecipientCategory `yaml:"recipient_categories"`
}

// ParsedRecipientPattern porte les champs dérivés d'une expression-pattern et
// les méthodes pour évaluer un score utilisateur.
type ParsedRecipientPattern struct {
	Raw          string // chaîne d'origine, conservée pour Compact
	Op           string
	Threshold    int
	Window       time.Duration
	PerMonth     bool
	IsEmailMatch bool
	Email        string
}

var (
	// Total : accepte " sur " (forme préférée) et " / " (rétrocompat).
	totalPatternRe    = regexp.MustCompile(`^\s*(<=|>=|<|>|=)\s*(\d+)\s+commandes?(?:\s+sur\s+|\s*/\s*)(\d+)\s*m\s*$`)
	perMonthPatternRe = regexp.MustCompile(`^\s*(<=|>=|<|>|=)\s*(\d+)\s+commandes?\s*/\s*mois\s+sur\s+(\d+)\s*m\s*$`)
	emailPatternRe    = regexp.MustCompile(`^\s*is\s+(\S+@\S+)\s*$`)
)

// ParseRecipientPattern parse une expression de pattern unique.
func ParseRecipientPattern(pattern string) (ParsedRecipientPattern, error) {
	s := strings.TrimSpace(pattern)
	if m := emailPatternRe.FindStringSubmatch(s); m != nil {
		return ParsedRecipientPattern{
			Raw:          s,
			IsEmailMatch: true,
			Email:        strings.ToLower(strings.TrimSpace(m[1])),
		}, nil
	}
	if m := perMonthPatternRe.FindStringSubmatch(s); m != nil {
		threshold, _ := strconv.Atoi(m[2])
		months, _ := strconv.Atoi(m[3])
		if months <= 0 {
			return ParsedRecipientPattern{}, fmt.Errorf("pattern %q : la fenêtre doit être > 0 mois", pattern)
		}
		return ParsedRecipientPattern{
			Raw:       s,
			Op:        m[1],
			Threshold: threshold,
			Window:    time.Duration(months) * 30 * 24 * time.Hour,
			PerMonth:  true,
		}, nil
	}
	if m := totalPatternRe.FindStringSubmatch(s); m != nil {
		threshold, _ := strconv.Atoi(m[2])
		months, _ := strconv.Atoi(m[3])
		if months <= 0 {
			return ParsedRecipientPattern{}, fmt.Errorf("pattern %q : la fenêtre doit être > 0 mois", pattern)
		}
		return ParsedRecipientPattern{
			Raw:       s,
			Op:        m[1],
			Threshold: threshold,
			Window:    time.Duration(months) * 30 * 24 * time.Hour,
		}, nil
	}
	return ParsedRecipientPattern{}, fmt.Errorf(
		"pattern invalide %q (attendu : %q, %q ou %q)",
		pattern,
		"<op> N commande[s] sur Tm",
		"<op> N commande[s]/mois sur Tm",
		"is <email>",
	)
}

// CompileConditions parse les patterns du YAML (`pattern`, `all`, `any`) et
// remplit `Conditions` + `Combinator`. Retourne une erreur si la définition
// est invalide ou ambiguë.
func (cat *RecipientCategory) CompileConditions() error {
	hasPattern := strings.TrimSpace(cat.Pattern) != ""
	hasAll := len(cat.All) > 0
	hasAny := len(cat.Any) > 0

	count := 0
	if hasPattern {
		count++
	}
	if hasAll {
		count++
	}
	if hasAny {
		count++
	}
	switch count {
	case 0:
		return fmt.Errorf("au moins une clé parmi `pattern`, `all`, `any` doit être renseignée")
	case 1:
		// ok
	default:
		return fmt.Errorf("une seule clé parmi `pattern`, `all`, `any` doit être renseignée")
	}

	var raws []string
	switch {
	case hasPattern:
		raws = []string{cat.Pattern}
		cat.Combinator = "all" // sans importance pour 1 condition
	case hasAll:
		raws = cat.All
		cat.Combinator = "all"
	case hasAny:
		raws = cat.Any
		cat.Combinator = "any"
	}

	cat.Conditions = make([]ParsedRecipientPattern, 0, len(raws))
	for _, raw := range raws {
		p, err := ParseRecipientPattern(raw)
		if err != nil {
			return err
		}
		cat.Conditions = append(cat.Conditions, p)
	}
	return nil
}

// WindowMonths retourne la fenêtre en mois (approximée à 30 jours/mois).
func (p *ParsedRecipientPattern) WindowMonths() int {
	return int(p.Window / (30 * 24 * time.Hour))
}

func (p *ParsedRecipientPattern) compareCount(value int) bool {
	switch p.Op {
	case "<":
		return value < p.Threshold
	case "<=":
		return value <= p.Threshold
	case ">":
		return value > p.Threshold
	case ">=":
		return value >= p.Threshold
	case "=":
		return value == p.Threshold
	}
	return false
}

// Match évalue si le score d'un user satisfait la condition.
//
// Sémantique selon le mode :
//   - mode total : score = nombre total de commandes ; renvoie `score op Threshold`.
//   - mode per-month : score = nombre de mois calendaires qui qualifient ;
//     renvoie true si tous les mois de la fenêtre qualifient.
//   - mode email : score = 1 si email matche, 0 sinon ; renvoie score == 1.
func (p *ParsedRecipientPattern) Match(score int) bool {
	if p.IsEmailMatch {
		return score == 1
	}
	if p.PerMonth {
		return score >= p.WindowMonths()
	}
	return p.compareCount(score)
}

// MatchPerMonthCount applique la règle au compteur d'un mois individuel.
// Utilisé en mode per-month pour décider si un mois donné qualifie.
func (p *ParsedRecipientPattern) MatchPerMonthCount(monthCount int) bool {
	return p.compareCount(monthCount)
}

// Compact retourne une forme courte de la condition pour l'UI.
func (p *ParsedRecipientPattern) Compact() string {
	if p.IsEmailMatch {
		return p.Email
	}
	months := p.WindowMonths()
	noun := "commande"
	if p.Threshold > 1 {
		noun = "commandes"
	}
	op := p.Op
	switch op {
	case "<=":
		op = "≤"
	case ">=":
		op = "≥"
	}
	if p.PerMonth {
		return fmt.Sprintf("%s%d %s/mois sur %d mois", op, p.Threshold, noun, months)
	}
	return fmt.Sprintf("%s%d %s sur %d mois", op, p.Threshold, noun, months)
}

// Match évalue si la catégorie matche pour un user, à partir des scores par
// condition (un score par élément de Conditions, dans le même ordre).
func (cat *RecipientCategory) Match(scores []int) bool {
	if len(cat.Conditions) == 0 || len(scores) != len(cat.Conditions) {
		return false
	}
	if cat.Combinator == "any" {
		for i := range cat.Conditions {
			if cat.Conditions[i].Match(scores[i]) {
				return true
			}
		}
		return false
	}
	// "all" (par défaut)
	for i := range cat.Conditions {
		if !cat.Conditions[i].Match(scores[i]) {
			return false
		}
	}
	return true
}

// Compact retourne le libellé court affiché à côté du nom dans la dropdown.
// Pour une catégorie multi-conditions, joint les libellés courts avec ET/OU.
func (cat *RecipientCategory) Compact() string {
	if len(cat.Conditions) == 0 {
		return ""
	}
	parts := make([]string, len(cat.Conditions))
	for i := range cat.Conditions {
		parts[i] = cat.Conditions[i].Compact()
	}
	if len(parts) == 1 {
		return parts[0]
	}
	sep := " ET "
	if cat.Combinator == "any" {
		sep = " OU "
	}
	return strings.Join(parts, sep)
}
