package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/pkg/mailer"
	"gorm.io/gorm"
)

// ---- Template helpers ----

// rubriqueAdministration : les écrans qui vivent dans l'espace
// d'administration. Une seule liste, lue par le menu latéral comme par le fil
// d'Ariane — deux listes tenues à la main auraient divergé au premier ajout.
func rubriqueAdministration(categorie string) bool {
	switch categorie {
	case "admin", "member", "distribution", "contract", "amapadmin":
		return true
	}
	return false
}

var funcMap = template.FuncMap{
	// Les numéros sont saisis à la main : d'un trait, espacés, parfois d'un
	// double espace. Les écrans mêlaient tous ces formats ; on les redit ici
	// deux chiffres par deux, comme on les prononce. Chaîne ou pointeur
	// indifféremment : les champs de modèle sont nullables, ceux des vues non.
	"telephone": func(v any) string {
		switch t := v.(type) {
		case string:
			return formatTelephone(t)
		case *string:
			if t == nil {
				return ""
			}
			return formatTelephone(*t)
		}
		return ""
	},
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	// Accepte indifféremment une chaîne ou un pointeur : les champs de modèle
	// sont nullables, ceux des vues ne le sont pas, et les deux passent par ce
	// helper.
	"nl2br": func(v any) template.HTML {
		var s string
		switch t := v.(type) {
		case string:
			s = t
		case *string:
			if t == nil {
				return ""
			}
			s = *t
		default:
			return ""
		}
		s = strings.ReplaceAll(s, "\r\n", "\n")
		return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>"))
	},
	"derefFloat": func(f *float64) float64 {
		if f == nil {
			return 0
		}
		return *f
	},
	"derefInt": func(i *int) string {
		if i == nil {
			return ""
		}
		return fmt.Sprintf("%d", *i)
	},
	"hasFlag": func(flags uint, f uint) bool { return flags&f != 0 },
	// deref64 : un pointeur de flottant rendu tel qu'on le saisirait — sans
	// zéros inutiles. Employé pour un pourcentage, où « 5 » vaut mieux que
	// « 5.000000 ».
	"deref64": func(f *float64) string {
		if f == nil {
			return ""
		}
		return strconv.FormatFloat(*f, 'f', -1, 64)
	},

	"derefUint": func(i *uint) string {
		if i == nil {
			return ""
		}
		return fmt.Sprintf("%d", *i)
	},
	"frenchDate": func(t *time.Time) string {
		if t == nil {
			return "—"
		}
		months := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
		days := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
		return fmt.Sprintf("%s %d %s %d", days[t.Weekday()], t.Day(), months[t.Month()], t.Year())
	},
	"qtDisplay": func(f *float64) string {
		if f == nil {
			return ""
		}
		v := *f
		if v == math.Trunc(v) {
			return fmt.Sprintf("%g", v)
		}
		for _, d := range []int{2, 3, 4, 8, 16} {
			n := v * float64(d)
			r := math.Round(n)
			if math.Abs(n-r) < 0.0001 {
				num := int(r)
				g := gcdInt(num, d)
				return fmt.Sprintf("%d/%d", num/g, d/g)
			}
		}
		return fmt.Sprintf("%g", v)
	},
	// initials : les initiales pour la pastille du menu de compte.
	//
	// Découpe en runes et non en octets : « %.1s » sur « Élise » couperait le
	// É en deux et afficherait un caractère de remplacement.
	// initialesDe : les initiales d'un nom complet, quand l'écran ne dispose
	// pas du prénom et du nom séparément — une demande d'adhésion n'en garde
	// qu'une seule chaîne.
	"initialesDe": func(complet string) string {
		champs := strings.Fields(complet)
		out := ""
		for _, mot := range champs {
			for _, r := range mot {
				out += strings.ToUpper(string(r))
				break
			}
			if len(out) == 2 {
				break
			}
		}
		if out == "" {
			return "?"
		}
		return out
	},

	"initials": func(first, last string) string {
		pick := func(s string) string {
			for _, r := range strings.TrimSpace(s) {
				return strings.ToUpper(string(r))
			}
			return ""
		}
		out := pick(first) + pick(last)
		if out == "" {
			return "?"
		}
		return out
	},
	// dansAdministration : cet écran fait-il partie de l'espace
	// d'administration ? Le fil d'Ariane s'en sert pour poser son premier
	// cran. Même réflexion que bodyExtraClass, et pour la même raison : les
	// gabarits communs servent aussi des structures sans rubrique.
	"dansAdministration": func(v any) bool {
		val := reflect.ValueOf(v)
		for val.Kind() == reflect.Pointer {
			if val.IsNil() {
				return false
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return false
		}
		user := val.FieldByName("User")
		cat := val.FieldByName("Category")
		if !user.IsValid() || !cat.IsValid() || cat.Kind() != reflect.String {
			return false
		}
		if user.Kind() == reflect.Pointer && user.IsNil() {
			return false
		}
		return rubriqueAdministration(cat.String())
	},

	// bodyExtraClass : la classe supplémentaire du corps de page.
	//
	// Passe par la réflexion parce que les gabarits communs servent des
	// structures qui n'ont pas toutes les mêmes champs : la fiche publique
	// d'un groupe n'a ni compte ni rubrique, et « {{if .User}} » y faisait
	// échouer le rendu — la page s'arrêtait au milieu, pour des visiteurs qui
	// n'étaient même pas connectés.
	"bodyExtraClass": func(v any) string {
		val := reflect.ValueOf(v)
		for val.Kind() == reflect.Pointer {
			if val.IsNil() {
				return ""
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return ""
		}
		// Les champs promus d'un PageData embarqué se lisent par leur nom.
		user := val.FieldByName("User")
		cat := val.FieldByName("Category")
		if !user.IsValid() || !cat.IsValid() || cat.Kind() != reflect.String {
			return ""
		}
		if user.Kind() == reflect.Pointer && user.IsNil() {
			return ""
		}
		if rubriqueAdministration(cat.String()) {
			return " ac-avec-lateral"
		}
		return ""
	},
	// currentUser : le compte connecté, ou nil quand la page n'en porte pas.
	//
	// Même raison que bodyExtraClass : la fiche publique d'un groupe se rend
	// avec une structure sans champ User, et « {{if .User}} » y interrompait le
	// rendu — la page s'arrêtait au milieu de son script, pour des visiteurs
	// qui n'étaient même pas connectés. Ce défaut est antérieur au menu de
	// compte ; il se voyait seulement plus tard dans la page.
	"currentUser": func(v any) *model.User {
		val := reflect.ValueOf(v)
		for val.Kind() == reflect.Pointer {
			if val.IsNil() {
				return nil
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return nil
		}
		f := val.FieldByName("User")
		if !f.IsValid() || f.Kind() != reflect.Pointer || f.IsNil() {
			return nil
		}
		u, _ := f.Interface().(*model.User)
		return u
	},
	// isAdminSection : cette page appartient-elle à l'administration ? C'est ce
	// qui décide d'afficher le menu latéral, et de décaler le contenu.
	"isAdminSection": func(category string) bool {
		switch category {
		case "admin", "member", "distribution", "contract", "amapadmin":
			return true
		}
		return false
	},
	// ouvrirProducteurs : la vue d'une distribution, en demandant que son volet
	// de producteurs paraisse déjà ouvert.
	//
	// Le bandeau porte les mêmes formes partout, mais pas les mêmes mots :
	// l'accueil annonce la récolte qui vient, « À propos » parle du groupe.
	// Un gabarit unique et deux jeux de mots, plutôt que deux copies du même
	// balisage qui auraient divergé à la première retouche.
	"bandeau": func(titre, texte string) BandeauView {
		return BandeauView{Titre: titre, Texte: texte}
	},

	// La carte mise en avant l'ouvre — c'est là qu'on veut voir ce qui sera
	// proposé, et un volet fermé cacherait justement ce qu'on est venu
	// regarder. Les distributions suivantes, déjà repliées, le gardent fermé :
	// deux niveaux de dépliage ouverts d'emblée noieraient la page.
	"ouvrirProducteurs": func(v MultiDistribView, ouvrir bool) MultiDistribView {
		v.OuvrirProducteurs = ouvrir
		return v
	},
	// teteAilleurs : la barre des producteurs est posée hors du corps — dans
	// l'en-tête de la carte — et le corps ne doit donc pas la répéter.
	"teteAilleurs": func(v MultiDistribView) MultiDistribView {
		v.TeteAilleurs = true
		return v
	},
	"paginateInts": paginateInts,
	"seq": func(start, end int) []int {
		s := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			s = append(s, i)
		}
		return s
	},
	// price formate un montant en omettant les centimes quand ils sont nuls :
	// « 2 » plutôt que « 2.00 », « 2.50 » inchangé.
	//
	// Séparateur décimal : le point, comme les `printf "%.2f"` qu'il remplace
	// dans les templates. Passer à la virgule changerait l'affichage de tous
	// les montants du site, ce qui n'est pas l'objet.
	"price": formatPrice,

	// priceFr : même règle, mais avec la virgule française. Réservé aux champs
	// de formulaire, qui l'utilisaient déjà.
	"priceFr": func(v float64) string {
		return strings.Replace(formatPrice(v), ".", ",", 1)
	},
}

// formatPrice rend un montant sans centimes superflus.
//
// La comparaison porte sur le montant ARRONDI au centime, pas sur la valeur
// brute : un float64 issu d'un calcul vaut souvent 1.9999999999 là où l'on
// attend 2, et un simple `v == math.Trunc(v)` afficherait alors « 2.00 ».
func formatPrice(v float64) string {
	cents := math.Round(v * 100)
	if math.Mod(cents, 100) == 0 {
		return strconv.FormatFloat(cents/100, 'f', 0, 64)
	}
	return strconv.FormatFloat(cents/100, 'f', 2, 64)
}

func loadTemplates(names ...string) (*template.Template, error) {
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = "templates/" + n
	}
	return template.New("").Funcs(funcMap).ParseFiles(paths...)
}

// ---- Common page data ----

type BreadcrumbItem struct {
	Name string
	Link string
}

type PageData struct {
	Title            string
	User             *model.User
	Group            *model.Group
	IsGroupManager   bool
	HasMembership    bool
	HasMessages      bool
	HasCatalogAdmin  bool
	HasDatabaseAdmin bool
	// Les deux délégations qui ont remplacé les anciens « droits
	// administrateur » : elles ouvrent un écran chacune, pas le groupe entier.
	HasDistributions bool
	HasParameters    bool
	// IsTechnicalManager : rôle unique de l'installation, tenu de la
	// configuration. Il vaut tous les droits sur tous les groupes.
	IsTechnicalManager bool
	// ShowVendorsTab : l'écran des producteurs est-il proposé ? Vrai pour qui a
	// une fonction dans le groupe, et pour tous si la configuration rouvre
	// l'onglet (ui.vendors_tab_for_members).
	ShowVendorsTab bool
	// CanEditVendors : peut creer, modifier et supprimer une fiche
	// producteur. Plus etroit que ShowVendorsTab, qui n'ouvre qu'une
	// lecture : une fiche ne porte pas de groupe, plusieurs peuvent
	// commander chez le meme producteur, et la corriger d'un cote la change
	// pour tous. Le responsable de groupe en repond, personne d'autre — le
	// responsable de catalogue tient ses produits, pas l'identite de la
	// ferme. C'est aussi la regle que l'API applique deja a POST
	// /api/groups/:id/vendors.
	CanEditVendors bool
	// CanManageRights : peut attribuer les droits — le responsable de groupe et
	// le responsable technique, personne d'autre.
	CanManageRights   bool
	AllowedCatalogIDs []uint // nil = tous (GroupManager ou CatalogAdmin global)
	Category          string
	Breadcrumb        []BreadcrumbItem
	Flash             string
	FlashError        bool
	Redirect          string
	Container         string
	HideNav           bool
	// impersonation (« se connecter en tant que »)
	IsImpersonating  bool
	ImpersonatorName string
	// home page
	// HeroDistrib : la distribution mise en avant, et NextDistribs celles qui
	// suivent, réduites à une ligne. L'accueil sert à commander maintenant,
	// pas à comparer six jeudis : la hiérarchie de l'écran suit celle de
	// l'usage. MultiDistribs reste entier — les modales de commande en ont
	// besoin, et l'API le sert tel quel.
	HeroDistrib   *MultiDistribView
	NextDistribs  []MultiDistribView
	Groups        []model.Group
	MultiDistribs []MultiDistribView
	OpenCatalogs  []model.Catalog
	// PendingGroups : groupes où l'utilisateur attend une décision. L'écran de
	// choix annonçait « vous n'appartenez à aucun groupe » à un candidat dont
	// la demande était pourtant partie, ce qui se lit comme un échec.
	PendingGroups []model.Group
	// contract_view page
	Catalog      *model.Catalog
	Products     []model.Product
	ProductViews []ProductView
	Vendor       *model.Vendor
	Contact      *model.User
	Distribs     []DistribView
	// shop page
	MultiDistribID uint
	TargetUserID   uint
	// account page
	Subscriptions []SubscriptionView
	RecentOrders  []RecentOrderView
	// member page
	Members []MemberView
	// distribution page (reuses MultiDistribs above but also:)
	AllDistribs []DistribAdminView
	PeriodLabel string
	// Périodes voisines, calculées à partir de celle qu'on regarde. Les flèches
	// portaient un offset fixe (-1 et 1) : elles avançaient d'un cran depuis
	// l'origine, puis rejouaient le même lien sans plus rien changer.
	PrevOffset int
	NextOffset int
	// amapadmin page
	Places       []model.Place
	Admins       []model.User
	AmapAdminTab string
	// AmapAdminTitre et AmapAdminChapeau : le titre de l'onglet ouvert et sa
	// phrase d'explication. Ici et non sur AmapAdminPageData : deux écrans des
	// paramètres composent leur contexte à partir d'un simple PageData, et la
	// coquille commune les lit sur tous.
	AmapAdminTitre   string
	AmapAdminChapeau string
	NbMembers        int
	NbActiveCatalogs int
	PublicGroupURL   string
	LogoURL          string
	// amap page
	Vendors     []model.Vendor
	AmapVendors []AmapVendorView
	// contractAdmin page
	AdminCatalogs []CatalogAdminRow
	// account page membership
	MembershipRenewalPeriod string
	// SuggestPhone : l'utilisateur n'a pas renseigné de téléphone, on le lui
	// suggère (cf. buildPageData).
	SuggestPhone bool
	// member page pagination
	TotalMembers int
	TotalPages   int
	CurrentPage  int
	// AdminTiles : les domaines d'administration ouverts à ce visiteur.
	//
	// Portées par PageData et non passées au gabarit par une fonction : les
	// écrans rendent des structures qui embarquent PageData — JoinRequestsData,
	// CyclesData… — et un helper appelé avec « . » y recevait la structure
	// dérivée, pas PageData. Le rendu échouait alors en silence, tronquant la
	// page au milieu du menu. Un champ, lui, est promu partout.
	AdminTiles []AdminTile
	// JoinRequestCount : demandes d'adhésion en attente. Le compteur est ce qui
	// fait revenir sur l'écran — un lien seul se laisse oublier, et une demande
	// oubliée est un adhérent qui ne vient jamais.
	JoinRequestCount int
	SearchQuery      string
	MemberFilter     string
	// AnneeCourante : l'année civile, pour intituler « Adhésion 2026 ». Elle
	// vient du serveur plutôt que d'une fonction de gabarit : un seul écran
	// s'en sert, et l'horloge n'a rien à faire dans un modèle.
	AnneeCourante int
}

// CanManageCatalog retourne true si l'utilisateur peut gérer le catalogue donné.
// estProducteurAutonome distingue le producteur de celui qui administre.
//
// Le raccourci « Gérer mes produits » ne vaut que pour lui : un gestionnaire
// du groupe, un responsable des distributions ou un titulaire du droit
// « catalogues » sur l'ensemble du groupe passent par l'espace
// d'administration, où cet écran figure déjà. Le leur montrer deux fois
// n'aiderait personne et brouillerait le sens du bouton.
//
// La distinction tient à la portée du droit : une liste de catalogues nommés
// désigne un producteur, l'absence de liste désigne un droit global.
func estProducteurAutonome(pd PageData) bool {
	if pd.IsGroupManager || pd.HasDistributions || !pd.HasCatalogAdmin {
		return false
	}
	return len(pd.AllowedCatalogIDs) > 0
}

// lienMesProduits mène le producteur là où il va travailler.
//
// Un seul catalogue : droit à ses produits. Une liste d'un seul élément lui
// demanderait un clic pour choisir ce qu'il n'a pas à choisir. Plusieurs :
// la liste, puisqu'il faut bien qu'il désigne lequel.
func lienMesProduits(ids []uint) string {
	if len(ids) == 1 {
		return fmt.Sprintf("/contractAdmin/products/%d", ids[0])
	}
	return "/contractAdmin"
}

// accesAdministration : à qui l'espace d'administration s'ouvre.
//
// Le producteur en est écarté bien qu'il détienne le droit « catalogues » :
// il n'y trouverait qu'un écran, celui de ses propres catalogues, que son
// raccourci lui donne déjà en un clic. Un espace d'administration d'une seule
// entrée n'administre rien.
func accesAdministration(pd PageData) bool {
	if estProducteurAutonome(pd) {
		return false
	}
	return pd.IsGroupManager || pd.HasMembership || pd.HasCatalogAdmin ||
		pd.HasDistributions || pd.HasParameters || pd.HasDatabaseAdmin
}

// EstProducteurAutonome, LienMesProduits et AccesAdministration : des méthodes
// et non des champs.
//
// Un champ se remplit quelque part, et ce quelque part finit par être oublié —
// un écran qui compose son PageData autrement aurait affiché un en-tête faux
// sans que rien ne le signale. La méthode, elle, répond toujours à partir des
// droits eux-mêmes.
//
// Récepteur valeur : les gabarits lisent ces vues comme des valeurs, dont ils
// ne peuvent pas prendre l'adresse.
func (pd PageData) EstProducteurAutonome() bool { return estProducteurAutonome(pd) }

func (pd PageData) LienMesProduits() string { return lienMesProduits(pd.AllowedCatalogIDs) }

func (pd PageData) AccesAdministration() bool { return accesAdministration(pd) }

func (pd *PageData) CanManageCatalog(catalogID uint) bool {
	if pd.IsGroupManager {
		return true
	}
	if !pd.HasCatalogAdmin {
		return false
	}
	if pd.AllowedCatalogIDs == nil {
		return true // droit global sur tous les catalogues
	}
	for _, id := range pd.AllowedCatalogIDs {
		if id == catalogID {
			return true
		}
	}
	return false
}

// BandeauView : les mots du bandeau d'accueil. Le gabarit en fournit la forme,
// chaque page ses phrases.
type BandeauView struct {
	Titre string
	Texte string
}

type MultiDistribView struct {
	ID              uint            `json:"id"`
	Place           string          `json:"place"`
	PlaceAddress    string          `json:"placeAddress"`
	DayOfWeek       string          `json:"dayOfWeek"`
	Day             string          `json:"day"`
	Month           string          `json:"month"`
	StartHour       string          `json:"startHour"`
	EndHour         string          `json:"endHour"`
	DayLabelFull    string          `json:"dayLabelFull"`
	Active          bool            `json:"active"`
	Past            bool            `json:"past"`
	CanOrder        bool            `json:"canOrder"`
	OrderNotYetOpen bool            `json:"orderNotYetOpen"`
	OrderStartDate  string          `json:"orderStartDate,omitempty"`
	OrderEndDate    string          `json:"orderEndDate,omitempty"`
	Distributions   bool            `json:"distributions"`
	UserOrders      []UserOrderView `json:"userOrders,omitempty"`
	UserOrderTotal  float64         `json:"userOrderTotal"`
	// Vendors : les producteurs présents ce jour-là, dédupliqués — deux
	// catalogues peuvent appartenir au même. « Qui sera là » est ce qui donne
	// envie d'ouvrir la boutique, plus qu'une date et une heure.
	Vendors       []VendorView       `json:"vendors,omitempty"`
	ProductImages []ProductImageView `json:"productImages,omitempty"`
	// Highlight : le libellé de mise en avant du premier catalogue qui en
	// porte un. Un seul, même si deux campagnes rares tombaient le même jour :
	// deux pastilles côte à côte se neutralisent, et il n'y a plus d'exception.
	Highlight       string `json:"highlight,omitempty"`
	VolunteerNeeded int    `json:"volunteerNeeded"`
	// VolunteerTotal et VolunteerFilled : les postes à tenir et ceux qui le
	// sont. Le seul écart ne suffisait pas à colorer la pastille — « il en
	// manque un » ne dit pas s'il en manque un sur deux ou un sur dix.
	VolunteerTotal  int `json:"volunteerTotal"`
	VolunteerFilled int `json:"volunteerFilled"`
	// VolunteerFrom et VolunteerTo bornent la semaine de cette distribution :
	// la pastille ouvre ainsi le calendrier des permanences là où elle pointe,
	// et non sur la semaine courante, qui peut être à des mois de distance.
	VolunteerFrom string `json:"-"`
	VolunteerTo   string `json:"-"`
	// UserIsVolunteer : le lecteur s'est inscrit pour tenir une permanence ce
	// jour-là. On le remercie au lieu de lui réclamer ce qu'il a déjà donné.
	UserIsVolunteer   bool   `json:"userIsVolunteer"`
	UserVolunteerRole string `json:"userVolunteerRole,omitempty"`
	// UserFirstName : le prénom du lecteur, pour le remercier par son nom quand
	// il tient une permanence. La vue le porte parce que le gabarit du corps ne
	// voit qu'elle, jamais le compte.
	UserFirstName string `json:"-"`
	// UserFullName : prénom et nom du lecteur, tels que la mention « Bénévole »
	// les affiche dans l'en-tête.
	UserFullName string `json:"-"`
	// OuvrirProducteurs : le volet des producteurs paraît-il déjà ouvert ?
	// Posé par le gabarit selon l'endroit où la distribution s'affiche, jamais
	// par les données.
	OuvrirProducteurs bool `json:"-"`
	// TeteAilleurs : la barre des producteurs est déjà affichée hors du corps —
	// dans l'en-tête de la carte — et le corps ne doit pas la répéter.
	TeteAilleurs   bool     `json:"-"`
	VolunteerRoles []string `json:"volunteerRoles,omitempty"`
}

type UserOrderView struct {
	ProductName string  `json:"productName"`
	SmartQty    string  `json:"smartQty"`
	UnitPrice   float64 `json:"unitPrice"`
	SubTotal    float64 `json:"subTotal"`
	Fees        float64 `json:"fees"`
	Total       float64 `json:"total"`
}

// VendorView : un producteur présent à une distribution, avec de quoi le
// présenter — la description que son responsable de catalogue a rédigée, et
// quelques-uns de ses produits.
//
// La description vient du producteur et non du catalogue : c'est la ferme
// qu'on présente, et elle ne change pas d'un contrat à l'autre.
type VendorView struct {
	ID          uint               `json:"id"`
	Name        string             `json:"name"`
	City        string             `json:"city,omitempty"`
	Organic     bool               `json:"organic"`
	Description string             `json:"description,omitempty"`
	Products    []ProductImageView `json:"products,omitempty"`
}

type ProductImageView struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	// HasPhoto : une photographie a réellement été importée — ni l'illustration
	// générique servie par défaut, ni un pictogramme déposé à sa place. La
	// bande de l'accueil ne montre que celles-là : une rangée de silhouettes
	// ne dit rien de ce qu'on trouvera.
	HasPhoto bool `json:"hasPhoto"`
	// Category : la sous-catégorie du produit, pour varier ce qu'on montre
	// d'un même producteur. Zéro quand il n'en a pas.
	Category uint `json:"-"`
	// FileID et Header servent à juger l'image sans la charger deux fois ; ils
	// ne quittent pas le serveur.
	FileID uint   `json:"-"`
	Header []byte `json:"-"`
}

type DistribView struct {
	Date  string
	Place string
}

type SubscriptionView struct {
	CatalogName string
	StartDate   string
	EndDate     string
}

type RecentOrderView struct {
	ProductName string
	CatalogName string
	SmartQty    string
	Total       float64
	Paid        bool
}

type MemberView struct {
	ID        uint
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Balance   float64
	IsManager bool
	Address   string
	// Adhésion de l'année courante : payée si MembershipFee > 0.
	// MembershipPaid n'a de sens que si Group.HasMembership.
	MembershipPaid bool
	MembershipFee  float64
}

type DistribAdminView struct {
	ID                   uint
	DayOfWeek            string
	Day                  string
	Month                string
	Date                 string
	DateISO              string // YYYY-MM-DD for URL
	StartHour            string
	EndHour              string
	Place                string
	PlaceAddress         string
	OrderStartDate       string
	OrderEndDate         string
	Catalogs             []string
	DistribLinks         []DistribLink
	Validated            bool
	NbOrders             int
	NbVolunteers         int
	NbVolunteersRequired int
	TotalAmount          float64
	IsFuture             bool
	IsOrderOpen          bool
	IsPast               bool
	IsToday              bool
}

type CatalogAdminRow struct {
	ID         uint
	VendorName string
	Name       string
	StartDate  string
	EndDate    string
	Active     bool
}

type DistribLink struct {
	DistribID        uint
	CatalogID        uint
	CatalogName      string
	VendorName       string
	CustomOrderStart string // si la date d'ouverture diffère du parent
	CustomOrderEnd   string // si la date de fermeture diffère du parent
}

// ---- Handler ----

type PagesHandler struct {
	db     *gorm.DB
	cfg    *config.Config
	mailer *mailer.Mailer
}

func NewPagesHandler(db *gorm.DB, cfg *config.Config) *PagesHandler {
	return &PagesHandler{db: db, cfg: cfg, mailer: mailer.New(cfg)}
}

// buildPageData charge User et Group depuis les claims.
// suggestPhone : faut-il rappeler à cet utilisateur de renseigner son numéro ?
//
// Le téléphone est ce par quoi on le joint quand quelque chose cloche pendant
// une distribution — un panier manquant, un retard, une absence. Le rappel se
// tait sur la page qui sert justement à le saisir : l'y afficher alors que le
// champ est sous les yeux n'aiderait personne.
func suggestPhone(u *model.User, path string) bool {
	if u == nil || strings.HasPrefix(path, "/account/edit") {
		return false
	}
	return u.Phone == nil || strings.TrimSpace(*u.Phone) == ""
}

func (h *PagesHandler) buildPageData(c *gin.Context) PageData {
	pd := PageData{}
	claims := middleware.GetClaims(c)
	if claims == nil {
		return pd
	}

	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err == nil {
		pd.User = &user
		pd.SuggestPhone = suggestPhone(&user, c.Request.URL.Path)
	}

	// Usurpation d'identité en cours : charge le nom du vrai utilisateur pour le bandeau.
	if claims.ImpersonatorID != 0 {
		pd.IsImpersonating = true
		var impersonator model.User
		if err := h.db.First(&impersonator, claims.ImpersonatorID).Error; err == nil {
			pd.ImpersonatorName = impersonator.FirstName + " " + impersonator.LastName
		}
	}

	if claims.GroupID != 0 {
		var group model.Group
		if err := h.db.Preload("Logo").First(&group, claims.GroupID).Error; err == nil {
			pd.Group = &group
			if group.Logo != nil {
				pd.LogoURL = FileURL(group.Logo.ID, h.cfg.Key, group.Logo.Name)
			}
		}
		// Check manager right (avec bypass admin site-wide).
		// groupAccess réutilise le cache posé par RequireGroupRight quand la
		// route porte le middleware d'autorisation (évite une 2e requête).
		ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
		if ug != nil {
			pd.IsGroupManager = ug.IsGroupManager()
			pd.HasMembership = pd.IsGroupManager || ug.HasRight(model.RightMembership)
			pd.HasMessages = pd.IsGroupManager || ug.HasRight(model.RightMessages)
			pd.HasCatalogAdmin = pd.IsGroupManager || ug.HasRight(model.RightCatalogAdmin)
			pd.HasDistributions = ug.CanManageDistributions()
			pd.HasParameters = ug.CanManageParameters()
			pd.IsTechnicalManager = pd.User != nil && isTechnicalManagerEmail(pd.User.Email)
			// Une fonction dans le groupe rend l'écran utile ; sinon il ne
			// s'ouvre que si la configuration le demande.
			pd.ShowVendorsTab = h.cfg.UI.VendorsTabForMembers ||
				pd.IsGroupManager || pd.HasCatalogAdmin ||
				pd.HasDistributions || pd.HasParameters || pd.HasMembership
			pd.CanEditVendors = pd.IsGroupManager
			pd.HasDatabaseAdmin = ug.CanAdminDatabase()
			pd.CanManageRights = ug.CanManageRights()
			// Composées ici pour que le menu latéral et l'écran d'accueil de
			// l'administration lisent la même source : deux listes tenues à la
			// main auraient divergé au premier ajout.
			pd.AdminTiles = adminTilesFor(pd)
			if pd.HasCatalogAdmin && !pd.IsGroupManager {
				for _, r := range ug.GetRights() {
					if r.Right == model.RightCatalogAdmin {
						if len(r.Params) == 0 {
							// droit global sur tous les catalogues
							pd.AllowedCatalogIDs = nil
						} else {
							for _, p := range r.Params {
								if id, err := strconv.ParseUint(p, 10, 64); err == nil {
									pd.AllowedCatalogIDs = append(pd.AllowedCatalogIDs, uint(id))
								}
							}
						}
					}
				}
			}
		}
	}
	return pd
}

// ---- Login page ----

func (h *PagesHandler) LoginPage(c *gin.Context) {
	redirect := c.Query("__redirect")
	if redirect == "" {
		redirect = "/user/choose"
	}
	t, err := loadTemplates("base.html", "cycles_style.html", "login.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	pd := PageData{Title: "Connexion", Redirect: redirect}

	pd.LogoURL = h.logoDuPortail()

	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Logout ----

func (h *PagesHandler) Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/user/login")
}

// ---- Impersonation (« se connecter en tant que ») ----

// MemberLoginAs bascule la session vers un autre membre du groupe courant.
// Protégé par le middleware reqMembership (gestionnaire ou droit « gestion des membres »).
func (h *PagesHandler) MemberLoginAs(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	targetID := uint(id)

	// La cible doit être membre du groupe courant de l'acteur (empêche d'usurper
	// un utilisateur arbitraire par son id).
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", targetID, claims.GroupID).First(&ug).Error; err != nil {
		c.Redirect(http.StatusFound, "/member")
		return
	}

	// Conserve l'usurpateur d'origine si déjà en usurpation, afin que « revenir »
	// ramène toujours au vrai compte.
	impersonator := claims.ImpersonatorID
	if impersonator == 0 {
		impersonator = claims.UserID
	}
	// Rien à faire si on cible son propre compte (réel ou usurpateur).
	if targetID == claims.UserID || targetID == impersonator {
		c.Redirect(http.StatusFound, "/member/view/"+c.Param("id"))
		return
	}

	token, err := h.issueTokenAs(targetID, claims.GroupID, impersonator)
	if err != nil {
		c.String(http.StatusInternalServerError, "erreur token")
		return
	}
	c.SetCookie("token", token, 3600*24*7, "/", "", false, true)
	c.Redirect(http.StatusFound, "/home")
}

// ImpersonateReturn met fin à une usurpation et restaure le compte usurpateur.
func (h *PagesHandler) ImpersonateReturn(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.ImpersonatorID == 0 {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	// Restaure le groupe courant s'il est toujours valide pour l'usurpateur
	// (le membre usurpé a pu changer de groupe entre-temps).
	returnGroup := claims.GroupID
	var ug model.UserGroup
	if h.db.Where("user_id = ? AND group_id = ?", claims.ImpersonatorID, returnGroup).First(&ug).Error != nil {
		returnGroup = 0
	}

	token, err := h.issueTokenAs(claims.ImpersonatorID, returnGroup, 0)
	if err != nil {
		c.String(http.StatusInternalServerError, "erreur token")
		return
	}
	c.SetCookie("token", token, 3600*24*7, "/", "", false, true)
	if returnGroup == 0 {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	c.Redirect(http.StatusFound, "/member")
}

// ---- Group selection ----

func (h *PagesHandler) ChoosePage(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	// If ?group=id → switch group and redirect to /home.
	// Le responsable technique peut basculer vers n'importe quel groupe ; les autres uniquement
	// vers les groupes dont ils sont membres.
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		var groupID uint
		if _, err := fmt.Sscanf(groupIDStr, "%d", &groupID); err == nil && groupID != 0 {
			allowed := false
			if isTechnicalManagerEmail(user.Email) {
				var g model.Group
				if h.db.First(&g, groupID).Error == nil {
					allowed = true
				}
			} else {
				var ug model.UserGroup
				if h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).First(&ug).Error == nil {
					allowed = true
				}
			}
			if allowed {
				newToken, err := h.issueTokenAs(claims.UserID, groupID, claims.ImpersonatorID)
				if err == nil {
					c.SetCookie("token", newToken, 3600*24*7, "/", "", false, true)
					c.Redirect(http.StatusFound, chooseDestination(c))
					return
				}
			}
		}
	}

	// Le responsable technique voit tous les groupes ; les autres uniquement les leurs.
	var groups []model.Group
	if isTechnicalManagerEmail(user.Email) {
		h.db.Preload("Logo").Order("name").Find(&groups)
	} else {
		var ugs []model.UserGroup
		h.db.Where("user_id = ?", claims.UserID).Find(&ugs)
		groupIDs := make([]uint, 0, len(ugs))
		for _, ug := range ugs {
			groupIDs = append(groupIDs, ug.GroupID)
		}

		if len(groupIDs) > 0 {
			h.db.Preload("Logo").Where("id IN ?", groupIDs).Find(&groups)
		}
	}
	// Un seul groupe accessible : il n'y a pas de choix à faire, on entre
	// directement dedans. Le token doit porter ce groupe avant la redirection,
	// sinon /home renvoie ici et la navigation boucle.
	if len(groups) == 1 {
		if claims.GroupID == groups[0].ID {
			c.Redirect(http.StatusFound, chooseDestination(c))
			return
		}
		newToken, err := h.issueTokenAs(claims.UserID, groups[0].ID, claims.ImpersonatorID)
		if err == nil {
			c.SetCookie("token", newToken, 3600*24*7, "/", "", false, true)
			c.Redirect(http.StatusFound, chooseDestination(c))
			return
		}
		// Émission impossible : on retombe sur la page de choix plutôt que de
		// boucler vers /home avec un token sans groupe.
	}

	logoURL := ""
	for _, g := range groups {
		if g.Logo != nil {
			logoURL = FileURL(g.Logo.ID, h.cfg.Key, g.Logo.Name)
			break
		}
	}

	t, err := loadTemplates("base.html", "design.html", "choose.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	var pending []model.Group
	for _, r := range PendingJoinRequestsFor(h.db, claims.UserID) {
		pending = append(pending, r.Group)
	}

	pd := PageData{
		Title:         "Choisir un groupe",
		User:          &user,
		Groups:        groups,
		PendingGroups: pending,
		HideNav:       true,
		LogoURL:       logoURL,
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// chooseDestination : où mener une fois le groupe courant fixé.
//
// L'accueil par défaut, mais l'écran demandé quand on y a été renvoyé faute de
// groupe courant : c'est ce qui permet à un lien reçu par courrier d'aboutir
// là où il promettait.
func chooseDestination(c *gin.Context) string {
	if dest := safeRedirectPath(c.Query("__redirect")); dest != "" {
		return dest
	}
	return "/home"
}

// ---- Home page ----

// Largeur de la fenêtre affichée par /home, en jours. Trois semaines : les
// distributions étant hebdomadaires, l'accueil montre trois commandes à la
// fois plutôt que deux.
const homePeriodDays = 21

// homePeriodData rassemble ce que l'accueil montre pour une période donnée.
//
// Extraite de HomePage pour que le défilement continu s'en serve : le fragment
// qui ajoute la période suivante doit produire exactement les mêmes vues, et
// une seconde implémentation aurait divergé au premier ajustement.
//
// Retourne nil quand il n'y a pas de quoi composer un accueil — ni compte, ni
// groupe courant. L'appelant décide alors où renvoyer le visiteur.
func (h *PagesHandler) homePeriodData(c *gin.Context, offsetWeeks int) *PageData {
	pd := h.buildPageData(c)
	// L'accueil respire, sans s'étaler : ses cartes se lisent l'une après
	// l'autre, et une colonne trop large les étire jusqu'à ce que l'œil doive
	// traverser l'écran pour relier un titre à son bouton. Plus large que la
	// grille d'origine, plus étroit que les écrans de gestion, dont les
	// tableaux profitent au contraire de la place.
	pd.Container = "container-fluid ac-accueil"
	if pd.User == nil || pd.Group == nil {
		return nil
	}
	pd.Title = "Accueil"
	pd.Category = "home"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Commandes", Link: "/home"}}

	claims := middleware.GetClaims(c)

	// Period navigation
	// Les flèches se déplacent depuis la période affichée, et non depuis
	// l'origine : avec un offset fixe, la seconde pression rejouait le même
	// lien et la page ne bougeait plus.
	pd.PrevOffset = offsetWeeks - 1
	pd.NextOffset = offsetWeeks + 1

	frMonthsFull := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	frDaysFull := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frDays := [...]string{"Dim", "Lun", "Mar", "Mer", "Jeu", "Ven", "Sam"}

	now := time.Now()
	// Fenêtre de 3 semaines démarrant au samedi précédent : à raison d'une
	// distribution hebdomadaire, la page affiche trois commandes par cran de
	// navigation. Les flèches se déplacent d'une fenêtre entière, d'où le même
	// pas des deux côtés.
	weekday := int(now.Weekday()) // 0=Sun
	daysSinceSat := (weekday + 1) % 7
	periodStart := now.AddDate(0, 0, -daysSinceSat+offsetWeeks*homePeriodDays)
	periodStart = time.Date(periodStart.Year(), periodStart.Month(), periodStart.Day(), 0, 0, 0, 0, periodStart.Location())
	periodEnd := periodStart.AddDate(0, 0, homePeriodDays)
	pd.PeriodLabel = fmt.Sprintf("Du %s %d %s %d au %s %d %s %d",
		frDays[periodStart.Weekday()], periodStart.Day(), frMonthsFull[periodStart.Month()], periodStart.Year(),
		frDays[periodEnd.Weekday()], periodEnd.Day(), frMonthsFull[periodEnd.Month()], periodEnd.Year(),
	)

	// Load upcoming MultiDistribs
	var distribs []model.MultiDistrib
	h.db.Where("group_id = ? AND distrib_start_date BETWEEN ? AND ?", pd.Group.ID, periodStart, periodEnd).
		Preload("Place").
		Preload("Distributions").
		Preload("Distributions.Catalog").
		Preload("Distributions.Catalog.Vendor").
		Order("distrib_start_date ASC").
		Find(&distribs)

	// Load all volunteer roles for the group
	var volRoles []model.VolunteerRole
	h.db.Where("group_id = ?", pd.Group.ID).Find(&volRoles)

	views := make([]MultiDistribView, 0, len(distribs))
	for _, md := range distribs {
		start := md.DistribStartDate
		end := md.DistribEndDate

		placeAddr := ""
		if md.Place.Address != nil {
			placeAddr = *md.Place.Address
		}
		if md.Place.ZipCode != nil {
			if placeAddr != "" {
				placeAddr += " "
			}
			placeAddr += *md.Place.ZipCode
		}
		if md.Place.City != nil {
			if placeAddr != "" {
				placeAddr += " "
			}
			placeAddr += *md.Place.City
		}

		view := MultiDistribView{
			ID:           md.ID,
			Place:        md.Place.Name,
			PlaceAddress: placeAddr,
			DayOfWeek:    frDaysFull[start.Weekday()],
			Day:          fmt.Sprintf("%d", start.Day()),
			Month:        frMonthsFull[start.Month()],
			StartHour:    fmt.Sprintf("%02d:%02d", start.Hour(), start.Minute()),
			EndHour:      fmt.Sprintf("%02d:%02d", end.Hour(), end.Minute()),
			DayLabelFull: fmt.Sprintf("%s %d %s à %02d:%02d", frDaysFull[start.Weekday()], start.Day(), frMonthsFull[start.Month()], start.Hour(), start.Minute()),
			Active:       now.After(start) && now.Before(end),
			Past:         !now.Before(time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())),
		}

		// Producteurs présents et leurs produits.
		//
		// Un passage sur les catalogues du jour : chacun désigne un producteur
		// et porte ses produits. Les producteurs sont dédupliqués — deux
		// catalogues peuvent venir de la même ferme — et leurs produits se
		// cumulent alors : on présente la ferme, pas le contrat.
		type ligneProduit struct {
			Name     string
			Category *uint
			FileID   *uint
			FileName *string
			Header   []byte
		}
		view.Highlight = highlightDesDistribs(md.Distributions)
		// Le producteur derrière la mise en avant : c'est lui qu'on épingle en
		// tête du volet, une fois les vignettes rapportées.
		var vendeurMisEnAvant uint
		for _, d := range md.Distributions {
			if d.Catalog.Highlight() != "" {
				vendeurMisEnAvant = d.Catalog.VendorID
				break
			}
		}

		parVendeur := make(map[uint]int, len(md.Distributions))
		candidats := make([][]ProductImageView, 0, len(md.Distributions))

		for _, d := range md.Distributions {
			v := d.Catalog.Vendor
			if v.ID == 0 {
				continue
			}
			idx, connu := parVendeur[v.ID]
			if !connu {
				vv := VendorView{ID: v.ID, Name: v.Name, Organic: v.Organic}
				if v.City != nil {
					vv.City = *v.City
				}
				if v.Description != nil {
					vv.Description = *v.Description
				}
				view.Vendors = append(view.Vendors, vv)
				candidats = append(candidats, nil)
				idx = len(view.Vendors) - 1
				parVendeur[v.ID] = idx
			}

			// On ratisse large — bien au-delà des six vignettes retenues.
			//
			// Les six premiers produits d'un catalogue viennent souvent du même
			// rayon : la Ferme du Jointout ouvre sur ses crottins, et ses cent
			// légumes attendent plus loin. Varier après coup un échantillon
			// déjà uniforme ne donne rien ; il faut d'abord voir large.
			//
			// Le coût reste modeste : la requête ne ramène qu'un nom, une
			// catégorie et quarante-huit octets d'en-tête par produit, sans
			// jamais charger d'image.
			var lignes []ligneProduit
			h.db.Table("products").
				Select("products.name, products.txp_sub_category_id AS category, "+
					"f.id AS file_id, f.name AS file_name, LEFT(f.data, 48) AS header").
				Joins("JOIN File f ON f.id = products.imageId").
				Where("products.catalog_id = ?", d.Catalog.ID).
				Order("products.id").
				Limit(catalogScanLimit).
				Scan(&lignes)

			for _, ln := range lignes {
				img := ProductImageView{
					Name: ln.Name,
					URL:  FileURL(*ln.FileID, h.cfg.Key, *ln.FileName),
				}
				if ln.Category != nil {
					img.Category = *ln.Category
				}
				// Le jugement de l'image attend : il peut coûter un décodage,
				// et l'immense majorité de ces candidats ne sera pas retenue.
				img.Header = ln.Header
				img.FileID = *ln.FileID
				candidats[idx] = append(candidats[idx], img)
			}
		}

		// Les rayons alternent avant qu'on ne choisisse, et non après : c'est
		// tout l'objet du ratissage ci-dessus.
		for idx := range view.Vendors {
			retenus := spreadCategories(dedupeFamilies(dedupeVariants(candidats[idx])))
			for _, img := range retenus {
				if len(view.Vendors[idx].Products) >= vendorProductCount {
					break
				}
				// Le décodage n'a lieu qu'ici, sur les quelques produits qui
				// ont une chance d'être montrés.
				if !h.isProductPhoto(img.FileID, img.Header) {
					continue
				}
				img.HasPhoto = true
				img.Header = nil
				view.Vendors[idx].Products = append(view.Vendors[idx].Products, img)
			}
		}

		// L'épinglage vient après l'attribution des vignettes : les index de
		// « candidats » suivent l'ordre de construction, et les brouiller
		// avant donnerait à chaque producteur les produits de son voisin.
		view.Vendors = epinglerEnTete(view.Vendors, vendeurMisEnAvant)

		// La bande de vignettes se compose une fois tous les producteurs
		// connus, sans requête supplémentaire : elle pioche dans ce qu'ils ont
		// déjà rapporté.
		view.ProductImages = pickAcrossVendors(view.Vendors)

		// L'état de commande se décide catalogue par catalogue, et non sur la
		// seule première distribution du jour : un producteur qui repousse sa
		// clôture rouvre ses commandes sans rouvrir celles de ses voisins. Il
		// suffit qu'un catalogue soit ouvert pour proposer le bouton ; le shop
		// n'ouvre ensuite que ceux qui le sont.
		if len(md.Distributions) > 0 {
			view.Distributions = true
			var latestEnd, nextStart *time.Time
			for _, d := range md.Distributions {
				d.MultiDistrib = md // CanOrderNow lit aussi les dates du jour
				if d.CanOrderNow() {
					view.CanOrder = true
					if end := d.EffectiveOrderEnd(); end != nil && (latestEnd == nil || end.After(*latestEnd)) {
						latestEnd = end
					}
					continue
				}
				if start := d.EffectiveOrderStart(); start != nil && now.Before(*start) {
					if nextStart == nil || start.Before(*nextStart) {
						nextStart = start
					}
				}
			}
			switch {
			case view.CanOrder && latestEnd != nil:
				view.OrderEndDate = frDateTimeLabel(*latestEnd)
			case !view.CanOrder && nextStart != nil:
				view.OrderNotYetOpen = true
				view.OrderStartDate = frDateTimeLabel(*nextStart)
			}
		}

		// Volunteer needs: count registered vs roles defined for this distrib's catalogs
		var nbRegistered int64
		h.db.Model(&model.Volunteer{}).Where("multi_distrib_id = ?", md.ID).Count(&nbRegistered)
		catalogIDs := make([]uint, 0, len(md.Distributions))
		for _, d := range md.Distributions {
			catalogIDs = append(catalogIDs, d.Catalog.ID)
		}
		rolesNeeded := make([]string, 0)
		for _, vr := range volRoles {
			if vr.CatalogID == nil {
				// Global role counts for this distrib
				rolesNeeded = append(rolesNeeded, vr.Name)
			} else {
				for _, cid := range catalogIDs {
					if *vr.CatalogID == cid {
						rolesNeeded = append(rolesNeeded, vr.Name)
						break
					}
				}
			}
		}
		// L'inscription du lecteur, s'il en a pris une : le bandeau lui dit
		// alors merci plutôt que de lui redemander de venir.
		var moi model.Volunteer
		if h.db.Where("multi_distrib_id = ? AND user_id = ?", md.ID, claims.UserID).
			First(&moi).Error == nil {
			view.UserIsVolunteer = true
			if moi.Role != nil {
				view.UserVolunteerRole = *moi.Role
			}
			if pd.User != nil {
				view.UserFirstName = pd.User.FirstName
				view.UserFullName = strings.TrimSpace(
					pd.User.FirstName + " " + pd.User.LastName)
			}
		}

		nbNeeded := len(rolesNeeded)
		// La semaine qui contient cette distribution, du dimanche au dimanche —
		// le découpage que le calendrier des permanences applique lui-même.
		jour := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		debutSemaine := jour.AddDate(0, 0, -int(jour.Weekday()))
		view.VolunteerFrom = debutSemaine.Format("2006-01-02")
		view.VolunteerTo = debutSemaine.AddDate(0, 0, 7).Format("2006-01-02")

		view.VolunteerTotal = nbNeeded
		view.VolunteerFilled = int(nbRegistered)
		if view.VolunteerFilled > nbNeeded {
			// Plus d'inscrits que de postes : la pastille dirait « 3 sur 2 ».
			view.VolunteerFilled = nbNeeded
		}
		if nbNeeded > int(nbRegistered) {
			view.VolunteerNeeded = nbNeeded - int(nbRegistered)
			view.VolunteerRoles = rolesNeeded
		}

		// Load user's orders for this MultiDistrib
		var orders []model.UserOrder
		h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
			Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
			Where("user_orders.user_id = ? AND multi_distribs.id = ?", claims.UserID, md.ID).
			Preload("Product").
			Find(&orders)

		for _, o := range orders {
			subTotal := o.Quantity * o.ProductPrice
			total := o.TotalPrice()
			view.UserOrders = append(view.UserOrders, UserOrderView{
				ProductName: o.Product.Name,
				SmartQty:    orderQtyLabel(o.Quantity, o.Product),
				UnitPrice:   o.ProductPrice,
				SubTotal:    subTotal,
				Fees:        total - subTotal,
				Total:       total,
			})
			view.UserOrderTotal += total
		}

		views = append(views, view)
	}
	pd.MultiDistribs = views
	pd.HeroDistrib, pd.NextDistribs = splitDistribs(views)

	// Load open variable-order catalogs
	var catalogs []model.Catalog
	h.db.Where("group_id = ? AND (end_date IS NULL OR end_date > ?) AND (start_date IS NULL OR start_date <= ?)",
		pd.Group.ID, time.Now(), time.Now()).
		Preload("Vendor").
		Find(&catalogs)
	for _, cat := range catalogs {
		if cat.Type == model.CatalogTypeVarOrder && cat.UsersCanOrder() {
			pd.OpenCatalogs = append(pd.OpenCatalogs, cat)
		}
	}

	return &pd
}

func (h *PagesHandler) HomePage(c *gin.Context) {
	offsetWeeks, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	pd := h.homePeriodData(c, offsetWeeks)
	if pd == nil {
		// La distinction se fait ici : pas de compte, on demande à se
		// connecter ; pas de groupe courant, on demande lequel.
		if middleware.GetClaims(c) == nil {
			c.Redirect(http.StatusFound, "/user/login?__redirect=/home")
		} else {
			c.Redirect(http.StatusFound, "/user/choose")
		}
		return
	}

	t, err := loadTemplates("base.html", "design.html", "home.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", *pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Contract view page ----

func (h *PagesHandler) ContractViewPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var catalog model.Catalog
	if err := h.db.Preload("Vendor").Preload("Contact").Preload("Products").Preload("Products.Image").
		First(&catalog, id).Error; err != nil {
		c.String(http.StatusNotFound, "catalogue introuvable")
		return
	}

	// Upcoming distributions
	var distribs []model.Distribution
	h.db.Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
		Where("distributions.catalog_id = ? AND multi_distribs.distrib_end_date >= ?", catalog.ID, time.Now()).
		Preload("MultiDistrib").
		Preload("MultiDistrib.Place").
		Order("multi_distribs.distrib_start_date ASC").
		Limit(10).
		Find(&distribs)

	distribViews := make([]DistribView, 0, len(distribs))
	for _, d := range distribs {
		distribViews = append(distribViews, DistribView{
			Date:  d.MultiDistrib.DistribStartDate.Format("02/01/2006"),
			Place: d.MultiDistrib.Place.Name,
		})
	}

	unitLabels := map[model.UnitType]string{
		model.UnitTypePiece:      "pièce",
		model.UnitTypeKilogram:   "kg",
		model.UnitTypeGram:       "g",
		model.UnitTypeLitre:      "L",
		model.UnitTypeCentilitre: "cl",
		model.UnitTypeMillilitre: "ml",
	}
	productViews := make([]ProductView, 0, len(catalog.Products))
	for _, p := range catalog.Products {
		imgURL := "/img/taxo/grey/fruits-legumes.png"
		if p.Image != nil {
			imgURL = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
		}
		ref := ""
		if p.Ref != nil {
			ref = *p.Ref
		}
		qt := 0.0
		if p.Qt != nil {
			qt = *p.Qt
		}
		// Les étiquettes de prix et de conditionnement, comme sur l'écran de
		// gestion : la fiche laissait ces deux champs vides, et la vignette
		// d'un produit s'affichait donc sans son prix.
		unit := unitLabels[p.UnitType]
		if unit == "" {
			unit = "pièce"
		}
		qtAffiche := qt
		if qtAffiche == 0 {
			qtAffiche = 1
		}
		productViews = append(productViews, ProductView{
			ID:            p.ID,
			Name:          p.Name,
			Ref:           ref,
			UnitType:      unit,
			QtLabel:       fmt.Sprintf("%s %s", floatToFractionStr(qtAffiche), unit),
			Price:         p.Price,
			PriceLabel:    formatPrice(p.Price) + " €",
			VAT:           p.VAT,
			Qt:            qt,
			Organic:       p.Organic,
			VariablePrice: p.VariablePrice,
			ImageURL:      imgURL,
		})
	}

	pd.Title = catalog.Name
	pd.Catalog = &catalog
	pd.Products = catalog.Products
	pd.ProductViews = productViews
	pd.Vendor = &catalog.Vendor
	pd.Contact = catalog.Contact
	pd.Distribs = distribViews

	// Même largeur que les autres écrans de gestion.
	pd.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "contract_view.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Account page ----

func (h *PagesHandler) AccountPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login?__redirect=/account")
		return
	}

	// Subscriptions AMAP
	if pd.Group != nil {
		var subs []model.Subscription
		h.db.Where("user_id = ?", pd.User.ID).
			Preload("Catalog").
			Find(&subs)
		for _, s := range subs {
			if s.Catalog.GroupID != pd.Group.ID {
				continue
			}
			sv := SubscriptionView{
				CatalogName: s.Catalog.Name,
				StartDate:   s.StartDate.Format("02/01/2006"),
			}
			if s.EndDate != nil {
				sv.EndDate = s.EndDate.Format("02/01/2006")
			}
			pd.Subscriptions = append(pd.Subscriptions, sv)
		}
	}

	// Recent variable orders (last 30 days)
	var orders []model.UserOrder
	h.db.Where("user_orders.user_id = ? AND user_orders.created_at >= ?", pd.User.ID, time.Now().AddDate(0, -1, 0)).
		Preload("Product").
		Preload("Product.Catalog").
		Order("user_orders.created_at DESC").
		Limit(20).
		Find(&orders)

	for _, o := range orders {
		// Tous les types de catalogue, et non les seules commandes libres :
		// l'écran ne retenait que celles-ci et annonçait « aucune commande » à
		// qui n'a que des contrats — c'est-à-dire à la plupart des adhérents.
		// Le catalogue est nommé pour qu'on distingue les unes des autres.
		pd.RecentOrders = append(pd.RecentOrders, RecentOrderView{
			ProductName: o.Product.Name,
			CatalogName: o.Product.Catalog.Name,
			SmartQty:    orderQtyLabel(o.Quantity, o.Product),
			Total:       o.TotalPrice(),
			Paid:        o.Paid,
		})
	}

	// Check membership renewal
	if pd.Group != nil && pd.Group.HasMembership {
		currentYear := time.Now().Year()
		var membership model.Membership
		if err := h.db.Where("user_id = ? AND group_id = ? AND year = ?", pd.User.ID, pd.Group.ID, currentYear).
			First(&membership).Error; err != nil {
			pd.MembershipRenewalPeriod = fmt.Sprintf("%d-%d", currentYear, currentYear+1)
		}
	}

	pd.Title = "Mon compte"
	pd.Category = "account"
	// Même colonne utile que la page des commandes.
	pd.Container = "container-fluid ac-accueil"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Mon compte", Link: "/account"}}

	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "account.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Account edit page ----

func (h *PagesHandler) AccountEditPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}
	pd.Title = "Modifier mon compte"
	pd.Category = "account"
	pd.Container = "container-fluid ac-accueil"
	pd.Breadcrumb = []BreadcrumbItem{
		{Name: "Mon compte", Link: "/account"},
		{Name: "Modifier", Link: ""},
	}
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "account_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Account update ----

func (h *PagesHandler) AccountUpdate(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	firstName := strings.TrimSpace(c.PostForm("firstName"))
	lastName := strings.TrimSpace(c.PostForm("lastName"))
	phone := strings.TrimSpace(c.PostForm("phone"))
	address1 := strings.TrimSpace(c.PostForm("address1"))
	zipCode := strings.TrimSpace(c.PostForm("zipCode"))
	city := strings.TrimSpace(c.PostForm("city"))

	updates := map[string]interface{}{
		"first_name": firstName,
		"last_name":  lastName,
	}
	if phone != "" {
		updates["phone"] = phone
	}
	if address1 != "" {
		updates["address1"] = address1
	}
	if zipCode != "" {
		updates["zip_code"] = zipCode
	}
	if city != "" {
		updates["city"] = city
	}

	h.db.Model(&model.User{}).Where("id = ?", pd.User.ID).Updates(updates)
	c.Redirect(http.StatusFound, "/account")
}

// ---- Member page (admin) ----

// filtreMembres applique le filtre demandé à la liste des membres.
//
// Ces filtres figuraient déjà comme liens dans l'écran, mais le paramètre
// n'était lu nulle part : ils ramenaient tous la liste entière. Les voici
// effectifs — un filtre qui ne filtre pas est pire que pas de filtre, il fait
// conclure à tort qu'aucun membre ne correspond.
//
// Les sous-requêtes évitent de multiplier les lignes : une jointure sur les
// paniers ferait apparaître un membre autant de fois qu'il a commandé.
func filtreMembres(q *gorm.DB, filtre string, groupID uint, annee int) *gorm.DB {
	const paniers = `SELECT 1 FROM baskets b
		JOIN multi_distribs md ON md.id = b.multi_distrib_id
		WHERE b.user_id = user_groups.user_id AND md.group_id = ?`
	const adhesion = `SELECT 1 FROM memberships m
		WHERE m.user_id = user_groups.user_id AND m.group_id = ? AND m.year = ?`

	switch filtre {
	case "withOrder":
		return q.Where("EXISTS ("+paniers+")", groupID)
	case "noOrder":
		return q.Where("NOT EXISTS ("+paniers+")", groupID)
	case "neverConnected":
		return q.Where("users.last_login IS NULL")
	case "upToDate":
		return q.Where("EXISTS ("+adhesion+")", groupID, annee)
	case "renewMembership":
		return q.Where("NOT EXISTS ("+adhesion+")", groupID, annee)
	}
	return q
}

// membresPerPage : la taille d'une fournée. Assez pour remplir un écran,
// assez petit pour que la suivante arrive avant qu'on l'attende.
const membresPerPage = 25

// chargerMembres remplit pd.Members pour une page donnée et rend l'effectif
// correspondant au filtre.
//
// Partagée par l'écran et par le fragment du défilement continu : deux
// implémentations auraient divergé au premier ajustement de tri ou de filtre,
// et la deuxième fournée n'aurait plus suivi la première.
// anneeCourante : l'année civile. Isolée pour que l'écran et son fragment la
// lisent au même endroit.
func anneeCourante() int { return time.Now().Year() }

func (h *PagesHandler) chargerMembres(pd *PageData, search, filtre string, page int) int64 {

	// Requête de base partagée pour le COUNT et le SELECT. La jointure sur les
	// utilisateurs est systématique : la recherche, le filtre « jamais
	// connecté » et le tri alphabétique en dépendent tous les trois.
	base := h.db.Model(&model.UserGroup{}).
		Joins("JOIN users ON users.id = user_groups.user_id").
		Where("user_groups.group_id = ?", pd.Group.ID)
	if search != "" {
		like := "%" + search + "%"
		base = base.Where("users.first_name LIKE ? OR users.last_name LIKE ? OR users.email LIKE ?",
			like, like, like)
	}
	base = filtreMembres(base, filtre, pd.Group.ID, time.Now().Year())

	var totalCount int64
	base.Count(&totalCount)

	// Tri par nom : sans ordre explicite, la base rend ce qu'elle veut, et la
	// page 2 pouvait redonner un membre déjà vu en page 1.
	var ugs []model.UserGroup
	base.Preload("User").
		Order("users.last_name, users.first_name").
		Offset((page - 1) * membresPerPage).Limit(membresPerPage).Find(&ugs)

	// Adhésions de l'année courante pour les membres affichés (un seul SELECT
	// IN au lieu d'une requête par ligne).
	feeByUserID := map[uint]float64{}
	if pd.Group.HasMembership && len(ugs) > 0 {
		userIDs := make([]uint, 0, len(ugs))
		for _, ug := range ugs {
			userIDs = append(userIDs, ug.UserID)
		}
		var ms []model.Membership
		h.db.Where("group_id = ? AND year = ? AND user_id IN ?",
			pd.Group.ID, time.Now().Year(), userIDs).Find(&ms)
		for _, m := range ms {
			feeByUserID[m.UserID] = m.Fee
		}
	}

	for _, ug := range ugs {
		addr := ""
		if ug.User.ZipCode != nil {
			addr = *ug.User.ZipCode
		}
		if ug.User.City != nil {
			if addr != "" {
				addr += " "
			}
			addr += *ug.User.City
		}
		if ug.User.Address1 != nil && addr != "" {
			addr = *ug.User.Address1 + " " + addr
		}
		fee, paid := feeByUserID[ug.UserID]
		pd.Members = append(pd.Members, MemberView{
			ID:        ug.User.ID,
			FirstName: ug.User.FirstName,
			LastName:  ug.User.LastName,
			Email:     ug.User.Email,
			Phone: func() string {
				if ug.User.Phone == nil {
					return ""
				}
				return *ug.User.Phone
			}(),
			Balance:        ug.Balance,
			IsManager:      ug.IsGroupManager(),
			Address:        addr,
			MembershipPaid: paid && fee > 0,
			MembershipFee:  fee,
		})
	}

	return totalCount
}

func (h *PagesHandler) MemberPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	if !pd.IsGroupManager && !pd.HasMembership {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	const perPage = membresPerPage
	pageStr := c.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	search := strings.TrimSpace(c.Query("q"))
	filtre := c.Query("filter")

	totalCount := h.chargerMembres(&pd, search, filtre, page)

	// Effectif total du groupe (indépendant d'une éventuelle recherche), pour
	// le libellé « Membres du groupe (N) » de la barre latérale.
	totalMembers := totalCount
	if search != "" {
		h.db.Model(&model.UserGroup{}).Where("group_id = ?", pd.Group.ID).Count(&totalMembers)
	}
	pd.TotalMembers = int(totalMembers)
	// Le nombre de pages ne sert plus qu'au repli sans JavaScript et à dire au
	// script s'il reste quelque chose à charger.
	totalPages := int(totalCount) / membresPerPage
	if int(totalCount)%membresPerPage != 0 {
		totalPages++
	}
	pd.TotalPages = totalPages
	pd.CurrentPage = page
	pd.SearchQuery = search
	pd.MemberFilter = filtre
	pd.AnneeCourante = anneeCourante()

	pd.JoinRequestCount = pendingJoinRequestCount(h.db, pd.Group.ID)

	// Même largeur que l'écran des commandes : les deux montrent des cartes
	// pleine largeur, et une colonne plus étroite ici étirerait le tableau
	// sur un fond vide de chaque côté.
	pd.Container = "container-fluid ac-accueil"
	pd.Title = "Membres"
	pd.Category = "member"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Membres", Link: "/member"}}

	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "member.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Distribution page (admin) ----

func (h *PagesHandler) DistributionPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	if !pd.HasDistributions {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	// Résultat d'une création par cycle. La phrase se compose ici, à partir de
	// deux compteurs, plutôt que d'arriver toute faite dans l'URL : un lien
	// forgé pourrait sinon faire afficher n'importe quel message.
	created, _ := strconv.Atoi(c.Query("created"))
	skipped, _ := strconv.Atoi(c.Query("skipped"))
	if created > 0 || skipped > 0 {
		pd.Flash = frCount(created, "distribution créée", "distributions créées") + "."
		if skipped > 0 {
			pd.Flash += " " + frCount(skipped, "date ignorée", "dates ignorées") +
				" : une distribution y était déjà programmée."
		}
	}

	// Period navigation
	offsetStr := c.DefaultQuery("offset", "0")
	offsetWeeks, _ := strconv.Atoi(offsetStr)
	// Les flèches se déplacent depuis la période affichée, et non depuis
	// l'origine : avec un offset fixe, la seconde pression rejouait le même
	// lien et la page ne bougeait plus.
	pd.PrevOffset = offsetWeeks - 1
	pd.NextOffset = offsetWeeks + 1

	frMonthsFull := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	frDaysFull := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}

	now := time.Now()
	periodStart := now.AddDate(0, 0, offsetWeeks*84-int(now.Weekday()))
	periodEnd := periodStart.AddDate(0, 0, 84)
	pd.PeriodLabel = fmt.Sprintf("Du %s %d %s %d au %s %d %s %d",
		frDaysFull[periodStart.Weekday()], periodStart.Day(), frMonthsFull[periodStart.Month()], periodStart.Year(),
		frDaysFull[periodEnd.Weekday()], periodEnd.Day(), frMonthsFull[periodEnd.Month()], periodEnd.Year(),
	)

	var mds []model.MultiDistrib
	h.db.Where("group_id = ? AND distrib_start_date BETWEEN ? AND ?", pd.Group.ID, periodStart, periodEnd).
		Preload("Place").
		Preload("Distributions.Catalog.Vendor").
		Order("distrib_start_date ASC").
		Find(&mds)

	for _, md := range mds {
		catalogs := make([]string, 0, len(md.Distributions))
		links := make([]DistribLink, 0, len(md.Distributions))
		fmtFR := func(t time.Time) string {
			return fmt.Sprintf("%s %d %s à %02d:%02d",
				frDaysFull[t.Weekday()], t.Day(), frMonthsFull[t.Month()],
				t.Hour(), t.Minute())
		}
		for _, d := range md.Distributions {
			catalogs = append(catalogs, d.Catalog.Name)
			link := DistribLink{
				DistribID:   d.ID,
				CatalogID:   d.CatalogID,
				CatalogName: d.Catalog.Name,
				VendorName:  d.Catalog.Vendor.Name,
			}
			if d.OrderStartDate != nil && md.OrderStartDate != nil && !d.OrderStartDate.Equal(*md.OrderStartDate) {
				link.CustomOrderStart = fmtFR(*d.OrderStartDate)
			} else if d.OrderStartDate != nil && md.OrderStartDate == nil {
				link.CustomOrderStart = fmtFR(*d.OrderStartDate)
			}
			if d.OrderEndDate != nil && md.OrderEndDate != nil && !d.OrderEndDate.Equal(*md.OrderEndDate) {
				link.CustomOrderEnd = fmtFR(*d.OrderEndDate)
			} else if d.OrderEndDate != nil && md.OrderEndDate == nil {
				link.CustomOrderEnd = fmtFR(*d.OrderEndDate)
			}
			links = append(links, link)
		}
		var nbOrders, nbVols int64
		h.db.Model(&model.UserOrder{}).
			Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
			Where("distributions.multi_distrib_id = ?", md.ID).
			Distinct("user_orders.user_id").
			Count(&nbOrders)
		h.db.Model(&model.Volunteer{}).Where("multi_distrib_id = ?", md.ID).Count(&nbVols)

		// Count required volunteer roles for this multidistrib's catalogs
		catalogIDs := make([]uint, 0, len(md.Distributions))
		for _, d := range md.Distributions {
			catalogIDs = append(catalogIDs, d.CatalogID)
		}
		var nbVolRoles int64
		if len(catalogIDs) > 0 {
			h.db.Model(&model.VolunteerRole{}).Where("group_id = ? AND catalog_id IN ?", md.GroupID, catalogIDs).Count(&nbVolRoles)
		}
		if nbVolRoles == 0 {
			nbVolRoles = 1
		}

		var orders []model.UserOrder
		var totalAmt float64
		h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
			Preload("Product").
			Where("distributions.multi_distrib_id = ?", md.ID).
			Find(&orders)
		for _, o := range orders {
			totalAmt += o.TotalPrice()
		}

		placeAddr := ""
		if md.Place.Address != nil {
			placeAddr = *md.Place.Address
		}
		if md.Place.ZipCode != nil {
			placeAddr += " " + *md.Place.ZipCode
		}
		if md.Place.City != nil {
			placeAddr += " " + *md.Place.City
		}

		orderStartStr, orderEndStr := "", ""
		if md.OrderStartDate != nil {
			orderStartStr = fmt.Sprintf("%s %d %s à %02d:%02d",
				frDaysFull[md.OrderStartDate.Weekday()], md.OrderStartDate.Day(),
				frMonthsFull[md.OrderStartDate.Month()], md.OrderStartDate.Hour(), md.OrderStartDate.Minute())
		}
		if md.OrderEndDate != nil {
			orderEndStr = fmt.Sprintf("%s %d %s à %02d:%02d",
				frDaysFull[md.OrderEndDate.Weekday()], md.OrderEndDate.Day(),
				frMonthsFull[md.OrderEndDate.Month()], md.OrderEndDate.Hour(), md.OrderEndDate.Minute())
		}

		pd.AllDistribs = append(pd.AllDistribs, DistribAdminView{
			ID:                   md.ID,
			DayOfWeek:            frDaysFull[md.DistribStartDate.Weekday()],
			Day:                  fmt.Sprintf("%d", md.DistribStartDate.Day()),
			Month:                frMonthsFull[md.DistribStartDate.Month()],
			Date:                 md.DistribStartDate.Format("02/01/2006"),
			DateISO:              md.DistribStartDate.Format("2006-01-02"),
			StartHour:            md.DistribStartDate.Format("15:04"),
			EndHour:              md.DistribEndDate.Format("15:04"),
			Place:                md.Place.Name,
			PlaceAddress:         placeAddr,
			OrderStartDate:       orderStartStr,
			OrderEndDate:         orderEndStr,
			Catalogs:             catalogs,
			DistribLinks:         links,
			Validated:            md.Validated,
			NbOrders:             int(nbOrders),
			NbVolunteers:         int(nbVols),
			NbVolunteersRequired: int(nbVolRoles),
			TotalAmount:          totalAmt,
			IsFuture:             md.DistribStartDate.After(now),
			IsPast:               md.DistribStartDate.Before(now),
			IsToday: func() bool {
				d := md.DistribStartDate
				return d.Year() == now.Year() && d.Month() == now.Month() && d.Day() == now.Day()
			}(),
			IsOrderOpen: func() bool {
				if md.OrderStartDate == nil || md.OrderEndDate == nil {
					return false
				}
				return now.After(*md.OrderStartDate) && now.Before(*md.OrderEndDate)
			}(),
		})
	}

	pd.Title = "Distributions"
	pd.Category = "distribution"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Distributions", Link: "/distribution"}}

	// Même largeur que les autres écrans de gestion.
	pd.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "distribution.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Amap page (producteurs) ----

type AmapCatalogView struct {
	ID            uint
	Name          string
	ProductImages []ProductImageView
	Coordinator   *model.User
}

type AmapVendorView struct {
	ID       uint
	Name     string
	City     string
	ImageURL string
	ZipCode  string
	Catalogs []AmapCatalogView
}

func (h *PagesHandler) AmapPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	// L'ecran ne sert qu'a qui administre le groupe ou ses catalogues : il
	// n'offre aucune action a un adherent, dont le menu ne le propose plus. La
	// porte se ferme aussi cote serveur, faute de quoi l'adresse resterait
	// accessible a qui la connait — et se rouvre par la meme condition que
	// l'onglet, pour qu'un groupe qui l'affiche puisse aussi y entrer.
	if !pd.ShowVendorsTab {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	// Load catalogs with vendor, contact, and product images
	var catalogs []model.Catalog
	h.db.Where("group_id = ?", pd.Group.ID).
		Preload("Vendor").
		Preload("Contact").
		Find(&catalogs)

	// Build vendor views ordered by first seen
	vendorOrder := []uint{}
	vendorMap := make(map[uint]*AmapVendorView)
	for _, cat := range catalogs {
		v := cat.Vendor
		if _, exists := vendorMap[v.ID]; !exists {
			vendorOrder = append(vendorOrder, v.ID)
			city := ""
			zip := ""
			if v.City != nil {
				city = *v.City
			}
			if v.ZipCode != nil {
				zip = *v.ZipCode
			}
			vendorMap[v.ID] = &AmapVendorView{
				ID:      v.ID,
				Name:    v.Name,
				City:    city,
				ZipCode: zip,
			}
		}
		// Load product images for this catalog (max 5)
		var prods []model.Product
		h.db.Where("catalog_id = ?", cat.ID).Preload("Image").Limit(5).Find(&prods)
		imgs := []ProductImageView{}
		for _, p := range prods {
			url := "/img/taxo/grey/fruits-legumes.png"
			if p.Image != nil {
				url = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
			}
			imgs = append(imgs, ProductImageView{URL: url, Name: p.Name})
		}
		catView := AmapCatalogView{
			ID:            cat.ID,
			Name:          cat.Name,
			ProductImages: imgs,
			Coordinator:   cat.Contact,
		}
		vendorMap[v.ID].Catalogs = append(vendorMap[v.ID].Catalogs, catView)
	}
	for _, id := range vendorOrder {
		pd.AmapVendors = append(pd.AmapVendors, *vendorMap[id])
	}

	// Les fiches que ce groupe a saisies et auxquelles aucun catalogue ne
	// repond encore. La liste se construit par jointure sur les catalogues :
	// sans ce rattrapage, un producteur tout juste cree disparaitrait de
	// l'ecran ou on vient de le saisir, et il faudrait connaitre son
	// adresse pour le retrouver. Ils ferment la liste, apres ceux qui
	// fournissent : c'est un debut de fiche, pas un fournisseur.
	var orphelins []model.Vendor
	h.db.Where("group_id = ?", pd.Group.ID).Order("name").Find(&orphelins)
	for _, v := range orphelins {
		if _, deja := vendorMap[v.ID]; deja {
			continue
		}
		city, zip := "", ""
		if v.City != nil {
			city = *v.City
		}
		if v.ZipCode != nil {
			zip = *v.ZipCode
		}
		pd.AmapVendors = append(pd.AmapVendors, AmapVendorView{
			ID: v.ID, Name: v.Name, City: city, ZipCode: zip,
		})
	}

	// Group contact principal
	if pd.Group.ContactID != nil {
		var contact model.User
		if err := h.db.First(&contact, *pd.Group.ContactID).Error; err == nil {
			pd.Contact = &contact
		}
	}

	pd.Title = "Producteurs"
	pd.Category = "amap"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Producteurs", Link: "/amap"}}

	// Rubrique et largeur : cet écran appartient à l'espace
	// d'administration, dont il doit garder le menu et la colonne.
	pd.Category = "contract"
	pd.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "amap.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- ContractAdmin page ----

func (h *PagesHandler) ContractAdminPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	if !pd.IsGroupManager && !pd.HasCatalogAdmin {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	frMonths := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	// La date seule, avec son année : la ligne d'un catalogue en côtoie
	// d'autres qui courent sur plusieurs saisons, et « Lundi 21 Novembre » ne
	// disait pas laquelle. L'heure de fin, elle, n'apprend rien dans une liste.
	frDate := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return fmt.Sprintf("%d %s %d", t.Day(), frMonths[t.Month()], t.Year())
	}

	var catalogs []model.Catalog
	h.db.Where("group_id = ?", pd.Group.ID).
		Preload("Vendor").
		Order("name ASC").
		Find(&catalogs)

	for _, cat := range catalogs {
		if !pd.CanManageCatalog(cat.ID) {
			continue
		}
		// Sans « du » ni « au » collés à la valeur : le gabarit les écrivait
		// aussi, et la ligne annonçait « du du Lundi 21 Novembre ».
		startStr := frDate(cat.StartDate)
		endStr := frDate(cat.EndDate)
		pd.AdminCatalogs = append(pd.AdminCatalogs, CatalogAdminRow{
			ID:         cat.ID,
			VendorName: cat.Vendor.Name,
			Name:       cat.Name,
			StartDate:  startStr,
			EndDate:    endStr,
			Active:     cat.IsActive(),
		})
	}

	pd.Title = "Catalogues"
	pd.Category = "contract"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Catalogues", Link: "/contractAdmin"}}

	// Même largeur que les autres écrans de gestion.
	pd.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "contract_admin.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- AmapAdmin page ----

func (h *PagesHandler) AmapAdminPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	// Le droit « paramètres » suffit à consulter : c'est le premier onglet de
	// sa rubrique, et le refuser en faisait le seul inaccessible. Les
	// paramètres ne s'y modifient que par le responsable du groupe — le
	// gabarit désactive le formulaire, et AmapAdminUpdate le refuse.
	if !pd.HasParameters {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	// Les listes que le formulaire des propriétés emploie.
	h.db.Where("group_id = ?", pd.Group.ID).Find(&pd.Places)
	h.db.Joins("JOIN user_groups ON user_groups.user_id = users.id").
		Where("user_groups.group_id = ? AND user_groups.rights LIKE ?", pd.Group.ID, "%GroupAdmin%").
		Order("users.last_name").Find(&pd.Admins)

	var nbMembers int64
	h.db.Model(&model.UserGroup{}).Where("group_id = ?", pd.Group.ID).Count(&nbMembers)
	pd.NbMembers = int(nbMembers)

	var nbActive int64
	now := time.Now()
	h.db.Model(&model.Catalog{}).
		Where("group_id = ? AND (end_date IS NULL OR end_date > ?) AND (start_date IS NULL OR start_date <= ?)",
			pd.Group.ID, now, now).
		Count(&nbActive)
	pd.NbActiveCatalogs = int(nbActive)

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	pd.PublicGroupURL = fmt.Sprintf("%s://%s/group/%d", scheme, c.Request.Host, pd.Group.ID)

	pd.Title = "Paramètres"
	pd.Category = "amapadmin"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}

	// Rubrique, largeur et fil : cet écran passe à côté de
	// buildAmapAdminData, il pose lui-même ce que celui-ci donne aux autres.
	pd.Category = "amapadmin"
	pd.Container = "container-fluid ac-accueil"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "amapadmin_layout.html", "amapadmin.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- AmapAdmin edit page (form) ----

// AmapAdminEditPage : les propriétés se règlent désormais sur le tableau de
// bord des paramètres, où elles voisinent l'identité du groupe qu'elles
// définissent. Cette adresse y ramène, pour les liens et les signets qui la
// portent encore.
func (h *PagesHandler) AmapAdminEditPage(c *gin.Context) {
	c.Redirect(http.StatusFound, "/amapadmin")
}

func (h *PagesHandler) AmapAdminUpdate(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	// Le responsable du groupe, et lui seul : ces réglages engagent le groupe
	// entier — son type, ses inscriptions, sa présence dans l'annuaire. Le
	// droit « paramètres » ouvrait cet enregistrement alors que le formulaire
	// lui était déjà refusé ; un POST forgé suffisait à le contourner.
	if !pd.IsGroupManager {
		c.String(http.StatusForbidden, "seul le responsable du groupe peut modifier ces paramètres")
		return
	}

	name := c.PostForm("name")
	txtIntro := c.PostForm("txt_intro")
	txtHome := c.PostForm("txt_home")
	txtDistrib := c.PostForm("txt_distrib")
	extURL := c.PostForm("ext_url")
	headEmail := strings.TrimSpace(c.PostForm("head_email"))
	groupType := c.PostForm("group_type")
	regOption := c.PostForm("reg_option")

	// Flags
	var flags uint
	if c.PostForm("flag_payments") == "1" {
		flags |= uint(model.GroupFlagHasPayments)
	}
	if c.PostForm("flag_network") == "1" {
		flags |= uint(model.GroupFlagCagetteNetwork)
	}
	if c.PostForm("flag_custom_categories") == "1" {
		flags |= uint(model.GroupFlagCustomizedCategories)
	}
	if c.PostForm("flag_hide_phone") == "1" {
		flags |= uint(model.GroupFlagHidePhone)
	}
	if c.PostForm("flag_phone_required") == "1" {
		flags |= uint(model.GroupFlagPhoneRequired)
	}
	if c.PostForm("flag_address_required") == "1" {
		flags |= uint(model.GroupFlagAddressRequired)
	}
	if c.PostForm("flag_shop_mode") == "1" {
		flags |= uint(model.GroupFlagShopMode)
	}

	updates := map[string]interface{}{
		"name":       name,
		"group_type": groupType,
		"reg_option": regOption,
		"flags":      flags,
	}
	if txtIntro != "" {
		updates["txt_intro"] = txtIntro
	} else {
		updates["txt_intro"] = nil
	}
	if txtHome != "" {
		updates["txt_home"] = txtHome
	} else {
		updates["txt_home"] = nil
	}
	if txtDistrib != "" {
		updates["txt_distrib"] = txtDistrib
	} else {
		updates["txt_distrib"] = nil
	}
	if extURL != "" {
		updates["ext_url"] = extURL
	} else {
		updates["ext_url"] = nil
	}
	// Vidé, le champ retombe sur l'adresse personnelle du responsable : c'est
	// ce que fait groupHeadEmail, et non l'absence de tout destinataire.
	if headEmail != "" {
		updates["head_email"] = headEmail
	} else {
		updates["head_email"] = nil
	}
	updates["head_email_include_account"] = c.PostForm("head_email_include_account") == "1"

	if cid, err := strconv.ParseUint(c.PostForm("contact_id"), 10, 64); err == nil && cid > 0 {
		updates["contact_id"] = uint(cid)
	} else {
		updates["contact_id"] = nil
	}
	var legalRepID uint
	if lid, err := strconv.ParseUint(c.PostForm("legal_representative_id"), 10, 64); err == nil && lid > 0 {
		legalRepID = uint(lid)
		updates["legal_representative_id"] = legalRepID
	} else {
		updates["legal_representative_id"] = nil
	}

	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(updates)

	if legalRepID > 0 {
		h.ensureGroupAdmin(pd.Group.ID, legalRepID)
	}

	c.Redirect(http.StatusFound, "/amapadmin")
}

// ensureGroupAdmin garantit que l'utilisateur donné possède le droit GroupAdmin
// sur le groupe indiqué. Utilisé pour le représentant légal.
func (h *PagesHandler) ensureGroupAdmin(groupID, userID uint) {
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&ug).Error; err != nil {
		return
	}
	rights := ug.GetRights()
	for _, r := range rights {
		if r.Right == model.RightGroupAdmin {
			return
		}
	}
	rights = append(rights, model.UserRight{Right: model.RightGroupAdmin})
	if raw, err := json.Marshal(rights); err == nil {
		ug.Rights = string(raw)
		h.db.Save(&ug)
	}
}

// ---- Helpers ----

func (h *PagesHandler) issueToken(userID, groupID uint) (string, error) {
	return h.issueTokenAs(userID, groupID, 0)
}

// issueTokenAs émet un token en conservant l'id de l'usurpateur (0 = pas d'usurpation).
func (h *PagesHandler) issueTokenAs(userID, groupID, impersonatorID uint) (string, error) {
	claims := &middleware.Claims{
		UserID:         userID,
		GroupID:        groupID,
		ImpersonatorID: impersonatorID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

// frMonthsLong nomme les mois pour frDateTimeLabel. Les handlers qui composent
// leurs propres libellés en gardent chacun une copie locale ; celle-ci est au
// niveau du paquet parce que le helper ci-dessous sert à plusieurs d'entre eux.
var frMonthsLong = [13]string{"", "Janvier", "Février", "Mars", "Avril", "Mai",
	"Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}

// frCount : "1 distribution créée", "3 distributions créées", "aucune
// distribution créée". Le zéro se dit, il ne s'écrit pas « 0 ».
func frCount(n int, singulier, pluriel string) string {
	switch {
	case n == 0:
		return "Aucune " + singulier
	case n == 1:
		return "1 " + singulier
	default:
		return fmt.Sprintf("%d %s", n, pluriel)
	}
}

// frDateTimeLabel : "Vendredi 12 Août à 18:00", tel qu'affiché aux membres
// pour l'ouverture et la clôture des commandes.
func frDateTimeLabel(t time.Time) string {
	return fmt.Sprintf("%s %d %s à %02d:%02d",
		frDays[t.Weekday()], t.Day(), frMonthsLong[t.Month()], t.Hour(), t.Minute())
}

// orderQtyLabel décrit ce qui a été commandé : le nombre de parts, puis le
// conditionnement tel qu'il est saisi sur la fiche produit — "3 × 500 g".
// Coller directement l'unité du produit sur le nombre de parts, sans tenir
// compte de Product.Qt, donnait "3 g" pour trois pains de 500 g.
// Un produit à la pièce sans conditionnement particulier n'a rien à multiplier :
// on écrit simplement "3 pièces".
func orderQtyLabel(quantity float64, p model.Product) string {
	qt := 1.0
	if p.Qt != nil && *p.Qt > 0 {
		qt = *p.Qt
	}
	// Le nombre de parts reste décimal : il s'additionne d'un membre à l'autre
	// sur les vues agrégées, où un total de 2,5 écrit en fraction donnerait
	// "5/2". Seul le conditionnement suit la notation en fractions de la fiche
	// produit, où "1/2 kg" est la façon habituelle de décrire une demi-part.
	count := strconv.FormatFloat(quantity, 'f', -1, 64)
	unit := unitLabelFor(p.UnitType)
	// unitLabelFor renvoie "pièce" aussi bien pour Piece que pour les unités
	// vides ou legacy : on teste le libellé plutôt que la constante. Seule la
	// pièce s'accorde ; les unités de mesure s'écrivent "500 g" au pluriel comme
	// au singulier.
	if unit == "pièce" {
		if qt == 1 {
			return count + " " + pluralPiece(quantity)
		}
		return count + " × " + floatToFractionStr(qt) + " " + pluralPiece(qt)
	}
	return count + " × " + floatToFractionStr(qt) + " " + unit
}

func pluralPiece(n float64) string {
	if n > 1 {
		return "pièces"
	}
	return "pièce"
}

// splitDistribs sépare la distribution à mettre en avant de celles qui suivent.
//
// La première ouverte à la commande l'emporte : c'est celle sur laquelle
// l'adhérent peut agir, et la mettre en avant lui épargne de chercher laquelle
// parmi une pile de journées se ressemblant toutes. À défaut — tout est fermé,
// ou rien n'ouvre encore — la plus proche tient la place, car l'écran ne doit
// jamais commencer par un vide.
func splitDistribs(views []MultiDistribView) (*MultiDistribView, []MultiDistribView) {
	if len(views) == 0 {
		return nil, nil
	}
	choisi := 0
	for i := range views {
		if views[i].CanOrder {
			choisi = i
			break
		}
	}
	hero := views[choisi]
	reste := make([]MultiDistribView, 0, len(views)-1)
	for i := range views {
		if i != choisi {
			reste = append(reste, views[i])
		}
	}
	return &hero, reste
}

// bandeTarget : combien de vignettes la bande cherche à montrer.
//
// Huit sur une carte large donne des vignettes d'environ 130 px — assez pour
// reconnaître un légume, assez peu pour ne pas transformer l'accueil en
// catalogue. Le compte s'écarte de cette cible quand la diversité l'exige :
// mieux vaut neuf vignettes couvrant neuf fermes que huit qui en cachent une.
const bandeTarget = 8

// catalogScanLimit : combien de produits d'un catalogue on examine avant de
// choisir. Assez large pour atteindre les rayons que l'ordre de création
// relègue au fond, assez borné pour que la requête reste courte.
const catalogScanLimit = 120

// vendorProductCount : combien de produits chaque producteur montre dans son
// volet. De quoi donner une idée sans transformer la page en catalogue.
const vendorProductCount = 6

// bandeMax borne l'écart : au-delà, les vignettes deviennent trop petites pour
// qu'on y distingue quoi que ce soit.
const bandeMax = 12

// epinglerEnTete remonte un producteur en première place, en préservant
// l'ordre des autres.
//
// La mise en avant annonce une campagne rare ; la laisser en cinquième
// position dans le volet reviendrait à l'annoncer puis à la cacher. Rendue
// telle quelle quand le producteur ne figure pas dans la liste — un catalogue
// mis en avant peut n'avoir aucun produit ce jour-là.
func epinglerEnTete(vendors []VendorView, id uint) []VendorView {
	if id == 0 || len(vendors) < 2 {
		return vendors
	}
	pos := -1
	for i := range vendors {
		if vendors[i].ID == id {
			pos = i
			break
		}
	}
	if pos <= 0 {
		return vendors
	}
	tete := vendors[pos]
	reste := append(vendors[:pos:pos], vendors[pos+1:]...)
	return append([]VendorView{tete}, reste...)
}

// highlightDesDistribs rend la mise en avant d'une journée : le libellé du
// premier catalogue qui en porte un, dans l'ordre où ils sont proposés.
//
// Un seul, et non la liste : deux pastilles côte à côte se neutralisent, et
// une journée où tout est exceptionnel n'a plus rien d'exceptionnel.
func highlightDesDistribs(ds []model.Distribution) string {
	for _, d := range ds {
		if h := d.Catalog.Highlight(); h != "" {
			return h
		}
	}
	return ""
}

// pickAcrossVendors choisit les produits de la bande en alternant les
// producteurs.
//
// Un tour donne à chacun son premier produit, le suivant son deuxième, et
// ainsi de suite. Prendre les huit premiers rencontrés — ce qui se faisait —
// les tirait presque tous du même catalogue : la bande montrait une ferme et
// taisait les cinq autres, alors qu'elle est là pour dire ce qu'on trouvera.
//
// Chaque producteur présent apparaît donc au moins une fois, tant qu'ils
// tiennent dans bandeMax.
func pickAcrossVendors(vendors []VendorView) []ProductImageView {
	if len(vendors) == 0 {
		return nil
	}

	// Un producteur par vignette au minimum : s'ils sont plus nombreux que la
	// cible, on l'élève pour n'en éclipser aucun, jusqu'à la limite.
	cible := bandeTarget

	// Seules les vraies photos entrent dans la bande : l'illustration générique
	// ne dit rien de ce qu'on trouvera, et une rangée de silhouettes grises
	// dessert la distribution qu'elle est censée annoncer.
	illustres := make([][]ProductImageView, 0, len(vendors))
	for _, v := range vendors {
		var avecPhoto []ProductImageView
		for _, p := range v.Products {
			if p.HasPhoto {
				avecPhoto = append(avecPhoto, p)
			}
		}
		if len(avecPhoto) > 0 {
			// Un seul conditionnement par produit, puis alternance des rayons :
			// l'ordre compte, dédupliquer après aurait pu vider une catégorie
			// de ce qui la représentait.
			illustres = append(illustres,
				spreadCategories(dedupeFamilies(dedupeVariants(avecPhoto))))
		}
	}
	if len(illustres) == 0 {
		return nil
	}

	if len(illustres) > cible {
		cible = len(illustres)
		if cible > bandeMax {
			cible = bandeMax
		}
	}

	out := make([]ProductImageView, 0, cible)
	for tour := 0; len(out) < cible; tour++ {
		ajoute := false
		for _, produits := range illustres {
			if tour >= len(produits) {
				continue
			}
			out = append(out, produits[tour])
			ajoute = true
			if len(out) == cible {
				break
			}
		}
		// Plus personne n'a de produit à ce rang : inutile d'aller plus loin.
		if !ajoute {
			break
		}
	}
	return out
}

// spreadCategories réordonne les produits d'un producteur pour que ses
// premiers montrent des rayons différents.
//
// Une ferme qui vend des fromages et vingt légumes voyait ses vignettes tirées
// du même rayon — la bande annonçait des légumes et taisait la crémerie, alors
// qu'elle est là pour dire l'étendue de ce qu'on trouvera.
//
// Un tour de table entre catégories, dans l'ordre où elles apparaissent : le
// premier produit de chacune, puis le deuxième, et ainsi de suite. Les produits
// sans catégorie forment un groupe comme les autres, plutôt que d'être écartés.
func spreadCategories(produits []ProductImageView) []ProductImageView {
	if len(produits) < 3 {
		return produits
	}

	ordre := make([]uint, 0, 4)
	groupes := make(map[uint][]ProductImageView, 4)
	for _, p := range produits {
		if _, vu := groupes[p.Category]; !vu {
			ordre = append(ordre, p.Category)
		}
		groupes[p.Category] = append(groupes[p.Category], p)
	}
	if len(ordre) < 2 {
		return produits
	}

	out := make([]ProductImageView, 0, len(produits))
	for tour := 0; len(out) < len(produits); tour++ {
		ajoute := false
		for _, cat := range ordre {
			g := groupes[cat]
			if tour < len(g) {
				out = append(out, g[tour])
				ajoute = true
			}
		}
		if !ajoute {
			break
		}
	}
	return out
}

// isProductPhoto décide si l'image d'un produit est une photographie.
//
// Deux examens, du moins cher au plus coûteux. L'en-tête suffit à écarter les
// pictogrammes, et il est déjà en main. Le reste — étiquettes, logos, visuels
// faits d'aplats — ne se voit qu'en regardant l'image, ce qui demande de la
// charger : on ne le fait qu'une fois par fichier, et le verdict est retenu.
//
// Une image de produit ne change pratiquement jamais ; la décoder à chaque
// affichage de l'accueil coûterait quelques millisecondes par vignette pour un
// résultat invariable.
func (h *PagesHandler) isProductPhoto(fileID uint, header []byte) bool {
	if !looksLikePhoto(header) {
		return false
	}
	if verdict, connu := PhotoVerdictCached(fileID); connu {
		return verdict
	}

	var f model.File
	if err := h.db.Select("id, data").First(&f, fileID).Error; err != nil {
		// Illisible : on garde, plutôt que d'écarter la photo d'un producteur
		// sur un incident de lecture.
		RememberPhotoVerdict(fileID, true)
		return true
	}
	verdict := IsPhotograph(f.Data)
	RememberPhotoVerdict(fileID, verdict)
	return verdict
}

// formatTelephone redit un numéro par paires de chiffres — « 03 85 72 30 92 ».
// Les numéros arrivent tels qu'ils ont été tapés : collés, espacés, pointillés,
// parfois avec un espace en trop, et la colonne en devenait illisible.
//
// Ce qui n'est pas un numéro français à dix chiffres ressort inchangé. Un
// numéro suisse — « 00 41 792377328 » — redécoupé par paires ne serait ni plus
// juste ni plus lisible ; mieux vaut ne pas y toucher que le mettre de travers.
func formatTelephone(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var d []byte
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			d = append(d, s[i])
		}
	}
	// « +33 6 83 15 30 40 » se lit ici sous sa forme nationale : c'est celle
	// que le reste de la colonne emploie.
	if strings.HasPrefix(s, "+33") && len(d) == 11 {
		d = append([]byte{'0'}, d[2:]...)
	}
	if len(d) != 10 {
		return s
	}
	var b strings.Builder
	for i := 0; i < 10; i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.Write(d[i : i+2])
	}
	return b.String()
}

// logoDuPortail : le logo à montrer sur les écrans où personne n'est encore
// connecté — connexion, inscription, mot de passe oublié.
//
// C'est celui du groupe, qui est aussi celui de l'association ayant conçu le
// logiciel. Aucun groupe n'est « le sien » à ce moment-là, mais un portail qui
// n'en héberge qu'un peut montrer le sien sans ambiguïté. Dès qu'il y en a
// deux, la page n'en affiche aucun plutôt que d'en élire un au hasard : la
// limite à deux suffit à trancher sans parcourir toute la table.
func (h *PagesHandler) logoDuPortail() string {
	var groupes []model.Group
	h.db.Preload("Logo").Limit(2).Find(&groupes)
	if len(groupes) == 1 && groupes[0].Logo != nil {
		return FileURL(groupes[0].Logo.ID, h.cfg.Key, groupes[0].Logo.Name)
	}
	return ""
}
