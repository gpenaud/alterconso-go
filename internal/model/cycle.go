package model

import (
	"fmt"
	"time"
)

// DistributionCycle : une série de distributions programmées ensemble.
//
// L'écran de programmation en série créait ses journées puis les oubliait :
// rien ne disait plus qu'elles formaient un même rythme. Le cycle donne un
// support à ce qui vaut pour la série entière plutôt que pour un jour — à
// commencer par le courrier qui annonce chaque ouverture de commandes.
//
// Il ne pilote pas les distributions : celles-ci gardent leurs propres dates,
// qu'un responsable peut décaler une à une. Le cycle dit d'où elles viennent,
// pas ce qu'elles doivent devenir.
type DistributionCycle struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	GroupID uint  `gorm:"not null;index" json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	PlaceID uint  `gorm:"not null" json:"-"`
	Place   Place `gorm:"foreignKey:PlaceID" json:"place"`

	// Nom donné au cycle. Sans lui, l'écran de configuration afficherait une
	// liste de dates que rien ne distingue quand un groupe en tient deux.
	Name string `gorm:"size:64;not null" json:"name"`

	// Périodicité. Deux unités, parce que deux natures de rythme :
	//
	// IntervalDays vaut pour les cycles courts (7, 14, 21 jours), qui tombent
	// toujours le même jour de la semaine — ce qui est justement ce qu'on
	// attend d'une distribution hebdomadaire.
	//
	// IntervalMonths vaut pour les cycles longs (1, 6, 12 mois). Les compter
	// en jours les ferait dériver : douze fois trente jours font onze mois et
	// demi, et un cycle annuel décalerait d'une semaine chaque année.
	// IntervalMonths > 0 l'emporte.
	IntervalDays   int `gorm:"not null" json:"intervalDays"`
	IntervalMonths int `gorm:"default:0" json:"intervalMonths"`

	StartDate time.Time `gorm:"not null" json:"startDate"`
	EndDate   time.Time `gorm:"not null" json:"endDate"`

	MultiDistribs []MultiDistrib `gorm:"foreignKey:CycleID" json:"-"`
}

func (c *DistributionCycle) TableName() string { return "distribution_cycles" }

// Next : la journée suivante, selon le rythme du cycle.
//
// Go normalise les dates impossibles : un cycle mensuel parti d'un 31 janvier
// passe au 3 mars plutôt qu'au 31 février. C'est le comportement de la
// bibliothèque standard, et le corriger demanderait de décider à la place du
// responsable si « le 31 » veut dire la fin du mois ou le trente-et-unième
// jour — ce qu'on lui laisse trancher en décalant la journée à la main.
func (c *DistributionCycle) Next(d time.Time) time.Time {
	if c.IntervalMonths > 0 {
		return d.AddDate(0, c.IntervalMonths, 0)
	}
	return d.AddDate(0, 0, c.IntervalDays)
}

// RhythmLabel dit la périodicité en toutes lettres : « 14 » ne se lit pas dans
// une liste, « Tous les quinze jours » si.
func (c *DistributionCycle) RhythmLabel() string {
	switch {
	case c.IntervalMonths == 1:
		return "Tous les mois"
	case c.IntervalMonths == 6:
		return "Tous les six mois"
	case c.IntervalMonths == 12:
		return "Tous les ans"
	case c.IntervalMonths > 1:
		return fmt.Sprintf("Tous les %d mois", c.IntervalMonths)
	case c.IntervalDays == 7:
		return "Toutes les semaines"
	case c.IntervalDays == 14:
		return "Tous les quinze jours"
	case c.IntervalDays == 21:
		return "Toutes les trois semaines"
	case c.IntervalDays == 30:
		return "Tous les mois"
	default:
		return fmt.Sprintf("Tous les %d jours", c.IntervalDays)
	}
}

// CycleMessage : le courrier qui annonce l'ouverture des commandes d'un cycle.
//
// Un seul par cycle : deux courriers pour une même ouverture ne diraient pas
// deux choses, ils arriveraient l'un après l'autre en disant la même.
//
// Il remplace le message d'ouverture figé dans le code pour les distributions
// du cycle. Celles qui n'appartiennent à aucun cycle, ou dont le cycle n'a pas
// de message actif, continuent de recevoir le gabarit d'origine — sans quoi
// activer cette fonction priverait de courrier les distributions isolées.
type CycleMessage struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"cdate"`
	UpdatedAt time.Time `json:"-"`

	CycleID uint              `gorm:"not null;uniqueIndex" json:"-"`
	Cycle   DistributionCycle `gorm:"foreignKey:CycleID" json:"-"`

	// Enabled : le message est écrit mais ne part pas encore. Un brouillon
	// vaut mieux qu'un texte qu'il faut effacer pour suspendre les envois.
	Enabled bool `gorm:"default:false" json:"enabled"`

	Subject string `gorm:"size:128;not null" json:"subject"`
	Body    string `gorm:"type:text;not null" json:"body"`

	// Image d'en-tête, stockée comme les logos et les photos de produits. Le
	// courrier la désigne par son URL signée : une image servie derrière un
	// cookie de session resterait un carré vide dans une boîte mail.
	ImageFileID *uint `json:"-"`
	ImageFile   *File `gorm:"foreignKey:ImageFileID" json:"-"`

	// Libellé du bouton qui ramène à l'espace de commande. Le lien lui-même
	// n'est pas configurable : il mène toujours à l'espace du destinataire, et
	// une adresse saisie à la main dans un courrier signé du groupe est
	// exactement ce dont on ne veut pas.
	LinkLabel string `gorm:"size:64" json:"linkLabel"`

	// Catégorie de destinataires, nommée comme dans la configuration. Vide, le
	// message suit la catégorie par défaut des notifications.
	RecipientCategory string `gorm:"size:64" json:"recipientCategory"`
}

func (m *CycleMessage) TableName() string { return "cycle_messages" }

// ButtonLabel : le libellé du bouton, ou celui par défaut s'il n'a pas été
// choisi. Un bouton sans texte ne se cliquerait pas.
func (m *CycleMessage) ButtonLabel() string {
	if m.LinkLabel == "" {
		return "Passer ma commande"
	}
	return m.LinkLabel
}

// IsSendable : le message peut-il partir ? Actif, et pourvu de ce qu'un
// courrier exige — un objet et un corps.
func (m *CycleMessage) IsSendable() bool {
	return m.Enabled && m.Subject != "" && m.Body != ""
}
