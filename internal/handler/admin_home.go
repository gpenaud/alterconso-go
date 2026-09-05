package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminTile : une fonction d'administration telle qu'elle s'affiche.
type AdminTile struct {
	Title string
	Desc  string
	Icon  string
	Link  string
	// Extras : les écrans secondaires du même domaine, offerts en raccourci.
	// Ils évitent d'imposer un détour par l'écran principal pour une action
	// qu'on vient précisément faire.
	Extras []AdminLink
	// Danger : ce domaine touche aux données sans garde-fou. La tuile s'en
	// distingue à l'œil, parmi d'autres qui se ressemblent toutes.
	Danger bool
	// Avertissement : ce qui peut mal tourner, écrit en toutes lettres. Une
	// couleur seule ne dit pas quoi, et ne se voit pas de tous.
	Avertissement string
	// HorsMenu : la tuile paraît sur la vue d'ensemble mais pas dans le menu
	// latéral. Pour les domaines qui vivent à l'intérieur d'un autre — les
	// droits sont un onglet des paramètres, une entrée de premier rang les
	// donnerait pour une rubrique à part.
	HorsMenu bool
}

type AdminLink struct {
	Name string
	Link string
}

type AdminHomeData struct {
	PageData
	Tiles []AdminTile
}

// AdminHomePage : le point d'entrée de l'administration.
//
// Les entrées d'administration ont quitté la barre principale, qui en portait
// huit et débordait. Elles se retrouvent ici, groupées par domaine et décrites
// — un menu ne dit que des noms, cet écran dit à quoi ils servent.
//
// Chaque bloc n'apparaît que si l'on en détient le droit : un délégataire ne
// voit que ce qu'il peut faire, plutôt qu'une liste dont la moitié lui serait
// refusée au clic.
func (h *PagesHandler) AdminHomePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	data := AdminHomeData{PageData: pd}
	data.Title = "Espace d'administration"
	data.Category = "admin"
	data.Container = "container-fluid ac-large"
	data.Tiles = pd.AdminTiles

	// Aucun droit : l'écran n'aurait rien à montrer, et son lien n'aurait pas
	// dû s'afficher. On renvoie à l'accueil plutôt que d'exposer une page vide.
	if len(data.Tiles) == 0 {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "admin_home.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// adminTilesFor compose l'écran selon les droits détenus.
//
// Isolée du handler pour être vérifiable sans base ni requête, comme authorize
// l'est pour les routes : c'est ici que se décide ce qu'un délégataire voit, et
// cette décision mérite d'être éprouvée directement.
//
// Chaque bloc n'apparaît qu'à qui peut s'en servir : afficher une entrée dont
// le clic mènerait à un refus est une promesse qu'on ne tient pas.
func adminTilesFor(pd PageData) []AdminTile {
	var tiles []AdminTile

	if pd.IsGroupManager || pd.HasMembership {
		tiles = append(tiles, AdminTile{
			Title: "Membres",
			Desc:  "L'annuaire du groupe, les adhésions et les paiements.",
			Icon:  "icon-users",
			Link:  "/member",
			Extras: []AdminLink{
				{Name: "Demandes d'adhésion", Link: "/member/requests"},
				{Name: "Nouveau membre", Link: "/member/insert"},
			},
		})
	}
	if pd.HasDistributions {
		tiles = append(tiles, AdminTile{
			Title: "Distributions",
			Desc:  "Le calendrier, les cycles et les permanences de bénévoles.",
			Icon:  "icon-calendar",
			Link:  "/distribution",
			Extras: []AdminLink{
				{Name: "Cycles et rappels", Link: "/distribution/cycles"},
				{Name: "Permanences", Link: "/distribution/volunteersParticipation"},
			},
		})
	}
	if pd.IsGroupManager || pd.HasCatalogAdmin {
		tiles = append(tiles, AdminTile{
			Title: "Catalogues",
			Desc:  "Les contrats avec les producteurs, leurs produits et leurs commandes.",
			Icon:  "icon-book",
			Link:  "/contractAdmin",
		})
	}
	if pd.ShowVendorsTab {
		tiles = append(tiles, AdminTile{
			Title: "Producteurs",
			Desc:  "Les fermes et artisans qui fournissent le groupe.",
			Icon:  "icon-farmer",
			Link:  "/amap",
		})
	}
	if pd.HasParameters {
		tiles = append(tiles, AdminTile{
			Title: "Paramètres",
			Desc:  "L'identité du groupe, ses adhésions, sa monnaie et ses documents.",
			Icon:  "icon-cog",
			Link:  "/amapadmin",
			Extras: []AdminLink{
				{Name: "Adhésions", Link: "/amapadmin/membership"},
				{Name: "Taux de TVA", Link: "/amapadmin/vatRates"},
				{Name: "Documents", Link: "/amapadmin/documents"},
			},
		})
	}
	if pd.CanManageRights {
		tiles = append(tiles, AdminTile{
			Title: "Droits",
			Desc:  "Qui administre quoi dans le groupe.",
			// La case cochée dit l'autorisation accordée. « icon-key »
			// n'existe pas dans la fonte du site : la tuile s'affichait sans
			// icône, et rien ne le signalait.
			Icon:     "icon-square-check",
			Link:     "/amapadmin/rights",
			HorsMenu: true,
		})
	}
	if pd.HasDatabaseAdmin {
		tiles = append(tiles, AdminTile{
			Title: "Base de données",
			Desc:  "L'édition directe des tables, sans les contrôles de l'application.",
			// La fonte du site n'a ni clé plate ni boîte à outils : l'engrenage
			// est ce qu'elle offre de plus proche d'un outil technique. Le
			// danger, lui, est porté par la couleur et par l'avertissement.
			Icon:   "icon-cog",
			Link:   "/admin/db",
			Danger: true,
			// Onglet des paramètres, comme les droits : pas d'entrée de
			// premier rang dans le menu latéral.
			HorsMenu: true,
			Avertissement: "Une modification faite ici ne se défait pas et " +
				"n'est vérifiée par rien : une commande, une adhésion ou un " +
				"compte peuvent disparaître sans avertissement.",
		})
	}
	return tiles
}
