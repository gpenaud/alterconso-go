package model

import "time"

// MultiDistrib regroupe plusieurs distributions (d'un même jour / même lieu).
type MultiDistrib struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"-"`

	GroupID uint  `json:"-"`
	Group   Group `gorm:"foreignKey:GroupID" json:"-"`

	PlaceID uint  `json:"-"`
	Place   Place `gorm:"foreignKey:PlaceID" json:"place"`

	DistribStartDate time.Time  `json:"distribStartDate"`
	DistribEndDate   time.Time  `json:"distribEndDate"`
	OrderStartDate   *time.Time `json:"orderStartDate,omitempty"`
	OrderEndDate     *time.Time `json:"orderEndDate,omitempty"`

	Validated bool `gorm:"default:false" json:"validated"`

	// CycleID : la série dont cette journée est issue, si elle en vient d'une.
	// Nullable, et il le restera : les distributions créées une par une n'ont
	// pas de cycle, et celles d'avant l'introduction des cycles non plus.
	CycleID *uint              `gorm:"index" json:"-"`
	Cycle   *DistributionCycle `gorm:"foreignKey:CycleID" json:"-"`

	Distributions []Distribution `gorm:"foreignKey:MultiDistribID" json:"-"`
}

func (m *MultiDistrib) TableName() string { return "multi_distribs" }

func (m *MultiDistrib) IsValidated() bool { return m.Validated }

// Distribution : une livraison pour un catalogue donné lors d'un MultiDistrib.
type Distribution struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"-"`

	CatalogID uint    `json:"-"`
	Catalog   Catalog `gorm:"foreignKey:CatalogID" json:"catalog"`

	MultiDistribID uint         `json:"-"`
	MultiDistrib   MultiDistrib `gorm:"foreignKey:MultiDistribID" json:"-"`

	// Dates spécifiques à cette distribution (surchargent celles du MultiDistrib si non nil)
	Date           *time.Time `json:"date,omitempty"`
	End            *time.Time `json:"end,omitempty"`
	OrderStartDate *time.Time `json:"orderStartDate,omitempty"`
	OrderEndDate   *time.Time `json:"orderEndDate,omitempty"`

	// Lieu (déprécié, utiliser MultiDistrib.Place)
	PlaceID *uint  `json:"-"`
	Place   *Place `gorm:"foreignKey:PlaceID" json:"-"`

	Orders []UserOrder `gorm:"foreignKey:DistributionID" json:"-"`
}

func (d *Distribution) TableName() string { return "distributions" }

// EffectiveDate retourne la date réelle (Distribution ou MultiDistrib).
func (d *Distribution) EffectiveDate() time.Time {
	if d.Date != nil {
		return *d.Date
	}
	return d.MultiDistrib.DistribStartDate
}

// MaxOrderEnd : la clôture la plus tardive qu'un producteur puisse se donner,
// fixée à la veille de la livraison.
//
// Repousser sa clôture sert à accepter une commande de dernière minute, pas à
// en accepter le matin même : il faut au producteur le temps de préparer ce
// qu'il a vendu.
func (d *Distribution) MaxOrderEnd() time.Time {
	return d.EffectiveDate().AddDate(0, 0, -1)
}

// MinOrderStart : l'ouverture la plus précoce qu'on puisse fixer — le début du
// jour en cours.
//
// Une ouverture datée d'hier ne veut rien dire : on ouvre les commandes
// maintenant ou plus tard, jamais rétroactivement. La borne tombe au début de
// la journée, et non à l'heure courante, pour qu'on puisse encore écrire « ce
// matin 8h » en réglant l'après-midi.
func MinOrderStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// derogates : une date propre déroge-t-elle à celle du jour commun ?
//
// Porter une valeur ne suffit pas : la quasi-totalité des distributions en
// portent une, recopiée du jour à leur création. Ce qui distingue une
// dérogation, c'est que la valeur diffère — sans quoi le signaler reviendrait
// à marquer tout le monde comme faisant exception.
func derogates(own, day *time.Time) bool {
	if own == nil {
		return false // suit le jour commun
	}
	if day == nil {
		return true // borne propre là où le jour n'en pose aucune
	}
	return !own.Equal(*day)
}

// DateDerogates : la livraison de ce catalogue diffère-t-elle du jour commun ?
func (d *Distribution) DateDerogates() bool {
	return d.Date != nil && !d.Date.Equal(d.MultiDistrib.DistribStartDate)
}

// OrderStartDerogates : idem pour l'ouverture des commandes.
func (d *Distribution) OrderStartDerogates() bool {
	return derogates(d.OrderStartDate, d.MultiDistrib.OrderStartDate)
}

// OrderEndDerogates : idem pour la clôture.
func (d *Distribution) OrderEndDerogates() bool {
	return derogates(d.OrderEndDate, d.MultiDistrib.OrderEndDate)
}

// EffectiveOrderEnd retourne la clôture qui s'applique : celle de la
// distribution quand elle en surcharge une, sinon celle du MultiDistrib.
// Nil : aucune clôture ne borne les commandes.
func (d *Distribution) EffectiveOrderEnd() *time.Time {
	if d.OrderEndDate != nil {
		return d.OrderEndDate
	}
	return d.MultiDistrib.OrderEndDate
}

// EffectiveOrderStart : même règle pour l'ouverture. Une distribution qui ne
// surcharge que sa clôture garde l'ouverture du MultiDistrib.
func (d *Distribution) EffectiveOrderStart() *time.Time {
	if d.OrderStartDate != nil {
		return d.OrderStartDate
	}
	return d.MultiDistrib.OrderStartDate
}

// OrderWindowStarted : la période de commande a-t-elle commencé ? Vrai aussi
// quand elle est déjà close — ce qui compte est qu'elle ne soit pas à venir.
//
// C'est la borne des interventions d'un gestionnaire : il corrige une commande
// pendant la période ou après coup, jamais avant que le catalogue ait ouvert.
// Rien ne serait « corrigé » sur une distribution que personne n'a encore pu
// commander ; ce serait commander à la place des membres, avant eux.
func (d *Distribution) OrderWindowStarted(now time.Time) bool {
	start := d.EffectiveOrderStart()
	return start == nil || !now.Before(*start)
}

// ShowsInShop : ce catalogue a-t-il sa place dans le rayon du shop ?
//
// Plus large que CanOrderNow, et c'est le point : un catalogue CLOS reste en
// rayon. L'en retirer le faisait disparaître sans un mot, et l'adhérent
// cherchait un producteur qui avait simplement fermé. Le shop l'affiche
// estompé, et refuse d'y toucher.
//
// Deux cas n'y figurent pas :
//   - le catalogue qui n'accepte pas la commande en ligne, qui n'a rien à y
//     faire à aucun moment ;
//   - celui dont l'ouverture n'est pas venue : l'annoncer d'avance égarerait,
//     et le dire « clos » serait faux.
func (d *Distribution) ShowsInShop(now time.Time) bool {
	return d.Catalog.UsersCanOrder() && d.OrderWindowStarted(now)
}

// CanOrderNow retourne true si les commandes sont ouvertes pour cette distribution.
func (d *Distribution) CanOrderNow() bool {
	if !d.Catalog.UsersCanOrder() {
		return false
	}
	orderEnd := d.EffectiveOrderEnd()
	if orderEnd == nil {
		return true
	}
	now := time.Now()
	if !now.Before(*orderEnd) {
		return false
	}
	// L'ouverture se lit séparément de la clôture. Les deux dates étaient prises
	// d'un seul bloc : une distribution surchargeant sa seule date de fin — ce
	// que fait la réouverture depuis la fiche catalogue — gardait alors une date
	// de début nulle, déréférencée juste après.
	orderStart := d.EffectiveOrderStart()
	return orderStart == nil || now.After(*orderStart)
}
