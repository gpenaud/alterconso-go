package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// ---- /account/quit ----

func (h *PagesHandler) AccountQuitPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	// Le départ ne se joue qu'en POST : servi en GET, il suffisait de charger
	// l'adresse — un lien, une image, un préchargeur de navigateur — pour être
	// retiré du groupe sans l'avoir demandé.
	if c.Request.Method == http.MethodPost {
		// Confirm quit: remove from group
		h.db.Where("user_id = ? AND group_id = ?", pd.User.ID, pd.Group.ID).
			Delete(&model.UserGroup{})
		// Reset JWT to groupId=0
		newToken, err := h.issueToken(pd.User.ID, 0)
		if err == nil {
			c.SetCookie("token", newToken, 3600*24*7, "/", "", false, true)
		}
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	type QuitData struct {
		PageData
	}
	data := QuitData{PageData: pd}
	data.Title = "Quitter le groupe"
	data.Category = "account"
	data.Container = "container-fluid ac-accueil"
	data.Breadcrumb = []BreadcrumbItem{
		{Name: "Mon compte", Link: "/account"},
		{Name: "Quitter le groupe", Link: ""},
	}

	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "account_quit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /member/invoice/:multiDistribId ----

type InvoiceData struct {
	GroupName    string
	MemberName   string
	MemberEmail  string
	MemberAddr   string
	Date         string
	Place        string
	VendorBlocks []InvoiceVendorBlock
	GrandTotal   float64
}

type InvoiceVendorBlock struct {
	VendorName string
	Lines      []InvoiceLine
	Total      float64
}

type InvoiceLine struct {
	SmartQty    string
	ProductName string
	Total       float64
}

func (h *PagesHandler) MemberInvoicePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	mdID, err := strconv.ParseUint(c.Param("multiDistribId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var md model.MultiDistrib
	if err := h.db.Preload("Place").Preload("Group").First(&md, mdID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	var orders []model.UserOrder
	h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ? AND user_orders.user_id = ?", mdID, pd.User.ID).
		Preload("Product").
		Preload("Product.Catalog").
		Preload("Product.Catalog.Vendor").
		Find(&orders)

	// Group by vendor
	vendorMap := make(map[uint]*InvoiceVendorBlock)
	vendorOrder := []uint{}
	var grandTotal float64

	for _, o := range orders {
		vid := o.Product.Catalog.VendorID
		if _, ok := vendorMap[vid]; !ok {
			vendorMap[vid] = &InvoiceVendorBlock{
				VendorName: o.Product.Catalog.Vendor.Name,
			}
			vendorOrder = append(vendorOrder, vid)
		}
		line := InvoiceLine{
			SmartQty:    orderQtyLabel(o.Quantity, o.Product),
			ProductName: o.Product.Name,
			Total:       o.TotalPrice(),
		}
		vendorMap[vid].Lines = append(vendorMap[vid].Lines, line)
		vendorMap[vid].Total += o.TotalPrice()
		grandTotal += o.TotalPrice()
	}

	blocks := make([]InvoiceVendorBlock, 0, len(vendorOrder))
	for _, vid := range vendorOrder {
		blocks = append(blocks, *vendorMap[vid])
	}

	addr := ""
	if pd.User.Address1 != nil {
		addr = *pd.User.Address1
	}
	if pd.User.ZipCode != nil {
		addr += " " + *pd.User.ZipCode
	}
	if pd.User.City != nil {
		addr += " " + *pd.User.City
	}

	data := InvoiceData{
		GroupName:    md.Group.Name,
		MemberName:   pd.User.FirstName + " " + pd.User.LastName,
		MemberEmail:  pd.User.Email,
		MemberAddr:   addr,
		Date:         md.DistribStartDate.Format("02/01/2006"),
		Place:        md.Place.Name,
		VendorBlocks: blocks,
		GrandTotal:   grandTotal,
	}

	t, err := loadTemplates("member_invoice.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "member_invoice", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /distribution/volunteersSummary/:id ----

type VolunteersSummaryData struct {
	PageData
	MultiDistrib model.MultiDistrib
	DateLabel    string
	RoleRows     []VolRoleAssignRow
}

type VolRoleAssignRow struct {
	RoleID      uint
	RoleName    string
	Members     []VolMemberOption
	AssignedUID uint // currently assigned user ID (0 = none)
}

type VolMemberOption struct {
	ID   uint
	Name string
}

func (h *PagesHandler) VolunteersSummaryPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var md model.MultiDistrib
	if err := h.db.Preload("Place").Preload("Distributions").First(&md, id).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	// Catalog IDs participating in this distribution
	catalogIDs := make([]uint, 0, len(md.Distributions))
	for _, d := range md.Distributions {
		catalogIDs = append(catalogIDs, d.CatalogID)
	}

	// Roles selected for this distribution (from multi_distrib_roles) OR all roles for its catalogs
	var roles []model.VolunteerRole
	if len(catalogIDs) > 0 {
		h.db.Where("group_id = ? AND catalog_id IN ?", pd.Group.ID, catalogIDs).Preload("Catalog").Find(&roles)
	}

	// Current volunteer assignments for this multidistrib
	var vols []model.Volunteer
	h.db.Where("multi_distrib_id = ?", md.ID).Find(&vols)
	// Map role name → assigned user ID
	roleAssign := map[string]uint{}
	for _, v := range vols {
		if v.Role != nil {
			roleAssign[*v.Role] = v.UserID
		}
	}

	// Members of the group for dropdown
	var ugs []model.UserGroup
	h.db.Where("group_id = ?", pd.Group.ID).Preload("User").Find(&ugs)
	members := make([]VolMemberOption, 0, len(ugs))
	for _, ug := range ugs {
		members = append(members, VolMemberOption{ID: ug.UserID, Name: ug.User.LastName + " " + ug.User.FirstName})
	}

	if c.Request.Method == http.MethodPost {
		// Réconciliation rôle par rôle, restreinte aux rôles réellement présents
		// dans le formulaire. Un DELETE global sur la multidistrib effacerait
		// aussi les inscriptions que cet écran n'affiche pas — celles prises via
		// le calendrier pour un rôle dont le catalogue ne participe plus, ou
		// sans rôle — alors que rien ici ne permet de les ressaisir.
		err := h.db.Transaction(func(tx *gorm.DB) error {
			for _, r := range roles {
				roleName := r.Name

				var uid uint
				if s := c.PostForm("role_" + strconv.Itoa(int(r.ID))); s != "" {
					if parsed, parseErr := strconv.ParseUint(s, 10, 64); parseErr == nil {
						uid = uint(parsed)
					}
				}

				// Titulaire inchangé : on ne touche pas à la ligne existante,
				// sa date d'inscription est conservée.
				if uid == roleAssign[roleName] {
					continue
				}

				// Le rôle change de titulaire (ou repasse à « non défini ») :
				// on retire l'ancien avant d'inscrire le nouveau.
				if err := tx.Where("multi_distrib_id = ? AND role = ?", md.ID, roleName).
					Delete(&model.Volunteer{}).Error; err != nil {
					return err
				}
				if uid == 0 {
					continue
				}
				if err := tx.Create(&model.Volunteer{
					UserID:         uid,
					MultiDistribID: md.ID,
					Role:           &roleName,
				}).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("[volunteers] enregistrement des rôles échoué (multiDistribID=%d): %v", md.ID, err)
			c.String(http.StatusInternalServerError, "enregistrement impossible")
			return
		}
		c.Redirect(http.StatusFound, "/distribution")
		return
	}

	frDays := []string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frMonths := []string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	dateLabel := frDays[md.DistribStartDate.Weekday()] + " " +
		strconv.Itoa(md.DistribStartDate.Day()) + " " +
		frMonths[md.DistribStartDate.Month()] + " à " +
		md.DistribStartDate.Format("15:04")

	data := VolunteersSummaryData{
		PageData:     pd,
		MultiDistrib: md,
		DateLabel:    dateLabel,
	}
	data.Title = "Bénévoles inscrits"

	for _, r := range roles {
		row := VolRoleAssignRow{
			RoleID:      r.ID,
			RoleName:    r.Name,
			Members:     members,
			AssignedUID: roleAssign[r.Name],
		}
		data.RoleRows = append(data.RoleRows, row)
	}

	// Même largeur que les autres écrans de gestion.
	data.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "distribution_volunteers_summary.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET /distribution/volunteers/:id/unregister ----

func (h *PagesHandler) VolunteerUnregisterPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	volID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	var vol model.Volunteer
	if err := h.db.First(&vol, volID).Error; err != nil {
		c.String(http.StatusNotFound, "inscription introuvable")
		return
	}
	if vol.UserID != pd.User.ID {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	mdID := vol.MultiDistribID
	h.db.Delete(&vol)
	c.Redirect(http.StatusFound, "/distribution/volunteersSummary/"+strconv.FormatUint(uint64(mdID), 10))
}

// ---- /distribution/volunteersParticipation ----

type VolParticipationData struct {
	PageData
	From    string
	To      string
	Members []VolParticipationRow
	// Tri : le classement demandé — "nom", "plus" ou "moins". Rendu au
	// gabarit pour qu'il marque la colonne active et propose l'inverse.
	Tri string
}

type VolParticipationRow struct {
	UserID   uint
	Name     string
	Done     int
	ToBeDone int
}

// trierParticipation classe les bénévoles.
//
// Par nom, c'est un annuaire : on y cherche quelqu'un. Par nombre de
// permanences, c'est une répartition de l'effort : on y voit qui porte le
// groupe et qui n'a pas encore eu l'occasion. Le nom départage les ex æquo,
// sans quoi l'ordre changerait d'un affichage à l'autre pour les nombreux
// bénévoles à zéro.
func trierParticipation(lignes []VolParticipationRow, tri string) []VolParticipationRow {
	out := make([]VolParticipationRow, len(lignes))
	copy(out, lignes)
	parNom := func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].UserID < out[j].UserID
	}
	switch tri {
	case "plus":
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Done != out[j].Done {
				return out[i].Done > out[j].Done
			}
			return parNom(i, j)
		})
	case "moins":
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Done != out[j].Done {
				return out[i].Done < out[j].Done
			}
			return parNom(i, j)
		})
	default:
		sort.SliceStable(out, parNom)
	}
	return out
}

func (h *PagesHandler) VolunteersParticipationPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
	toStr := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	if from.IsZero() {
		from = time.Now().AddDate(-1, 0, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}

	// Load all group members
	var ugs []model.UserGroup
	h.db.Where("group_id = ?", pd.Group.ID).Preload("User").Find(&ugs)

	// Count distributions in period (toBeDone = total distribs)
	var nbMDs int64
	h.db.Model(&model.MultiDistrib{}).
		Where("group_id = ? AND distrib_start_date BETWEEN ? AND ?", pd.Group.ID, from, to).
		Count(&nbMDs)

	data := VolParticipationData{PageData: pd, From: fromStr, To: toStr}
	data.Title = "Participation aux permanences"
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Distributions", Link: "/distribution"}}

	for _, ug := range ugs {
		// Count volunteer entries for this user in period
		var done int64
		h.db.Model(&model.Volunteer{}).
			Joins("JOIN multi_distribs ON multi_distribs.id = volunteers.multi_distrib_id").
			Where("volunteers.user_id = ? AND multi_distribs.group_id = ? AND multi_distribs.distrib_start_date BETWEEN ? AND ?",
				ug.UserID, pd.Group.ID, from, to).
			Count(&done)

		data.Members = append(data.Members, VolParticipationRow{
			UserID:   ug.UserID,
			Name:     ug.User.FirstName + " " + ug.User.LastName,
			Done:     int(done),
			ToBeDone: int(nbMDs),
		})
	}

	tri := c.Query("tri")
	if tri != "plus" && tri != "moins" {
		tri = "nom"
	}
	data.Tri = tri
	data.Members = trierParticipation(data.Members, tri)

	// Même largeur que les autres écrans de gestion.
	data.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "distribution_volunteers_participation.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /amapadmin/rights ----

type AmapAdminRightsData struct {
	AmapAdminPageData
	RightUsers []RightUserView

	// Message annonçant qu'un rôle à titulaire unique a changé de mains.
	Transfert string
}

type RightUserView struct {
	UserID uint
	Name   string
	Rights []string
	// IsTechnicalManager : rôle tenu de la configuration, non modifiable ici.
	IsTechnicalManager bool
}

func formatRightLabels(rights []model.UserRight, catalogMap map[string]string) []string {
	// Le responsable de groupe a tous les droits : les énumérer n'apprendrait
	// rien, et les délégations qu'il porterait par ailleurs sont redondantes.
	for _, r := range rights {
		if r.Right == model.RightGroupAdmin {
			return []string{r.Right.Label()}
		}
	}
	var labels []string
	for _, r := range rights {
		switch r.Right {
		case model.RightMembership, model.RightMessages,
			model.RightDistributions, model.RightParameters:
			labels = append(labels, r.Right.Label())
		case model.RightCatalogAdmin:
			if len(r.Params) == 0 {
				labels = append(labels, "Gestion des catalogues : tous")
			} else {
				for _, p := range r.Params {
					name, ok := catalogMap[p]
					if !ok {
						name = "Catalogue #" + p
					}
					labels = append(labels, "Catalogue : "+name)
				}
			}
		}
	}
	return labels
}

func (h *PagesHandler) AmapAdminRightsPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "rights")
	if !ok {
		return
	}

	// Posé par les formulaires quand un rôle à titulaire unique vient de
	// changer de mains : celui qui le perd ne l'apprendrait pas autrement.
	transfert := c.Query("transfert")

	var ugs []model.UserGroup
	h.db.Where("group_id = ?", base.Group.ID).Preload("User").Find(&ugs)

	var catalogs []model.Catalog
	h.db.Where("group_id = ?", base.Group.ID).Find(&catalogs)
	catalogMap := make(map[string]string, len(catalogs))
	for _, cat := range catalogs {
		catalogMap[strconv.FormatUint(uint64(cat.ID), 10)] = cat.Name
	}

	data := AmapAdminRightsData{AmapAdminPageData: base, Transfert: transfert}
	data.Title = "Droits d'administration"

	for _, ug := range ugs {
		rights := ug.GetRights()
		isTech := isTechnicalManagerEmail(ug.User.Email)
		// Le responsable technique est toujours listé, même sans droits
		// persistés : loadGroupAccess les lui accorde tous à la volée.
		if len(rights) == 0 && !isTech {
			continue
		}
		// Son rôle ne se confond avec aucun de ceux du groupe : il les vaut
		// tous, sur tous les groupes, et vient de la configuration. L'énumérer
		// parmi les droits du groupe le ferait passer pour le responsable de
		// CELUI-CI, place qui revient à un membre.
		var labels []string
		if isTech {
			labels = []string{model.LabelTechnicalManager}
		} else {
			labels = formatRightLabels(rights, catalogMap)
		}
		rv := RightUserView{
			UserID:             ug.UserID,
			Name:               ug.User.FirstName + " " + ug.User.LastName,
			Rights:             labels,
			IsTechnicalManager: isTech,
		}
		data.RightUsers = append(data.RightUsers, rv)
	}

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_rights.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET+POST /group/create/ ----

type GroupCreateData struct {
	PageData
	Error string
}

func (h *PagesHandler) GroupCreatePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	data := GroupCreateData{PageData: pd}
	data.Title = "Créer un groupe"

	if c.Request.Method == http.MethodPost {
		name := c.PostForm("name")
		groupType := c.PostForm("groupType")
		if name == "" {
			data.Error = "Le nom du groupe est obligatoire."
		} else {
			g := model.Group{
				Name:      name,
				GroupType: model.GroupType(groupType),
				RegOption: model.RegOptionOpen,
				Currency:  "€",
			}
			if err := h.db.Create(&g).Error; err != nil {
				data.Error = "Erreur lors de la création du groupe."
			} else {
				// Add creator as admin
				h.db.Create(&model.UserGroup{UserID: pd.User.ID, GroupID: g.ID})
				// Issue new token with this group
				newToken, err := h.issueToken(pd.User.ID, g.ID)
				if err == nil {
					c.SetCookie("token", newToken, 3600*24*7, "/", "", false, true)
				}
				c.Redirect(http.StatusFound, "/amapadmin")
				return
			}
		}
	}

	t, err := loadTemplates("base.html", "design.html", "group_create.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET+POST /amapadmin/rights/add ----

type AmapAdminRightsAddData struct {
	AmapAdminPageData
	Members  []model.UserGroup
	Catalogs []model.Catalog
	Error    string
	Success  string

	// Titulaire actuel du rôle à titulaire unique, pour prévenir que l'accorder
	// ici le lui retirera.
	GroupAdminHolder string

	// CanAssignGroupHead : le rôle de responsable de groupe ne se donne que par
	// le responsable technique. Le responsable en place distribue les autres
	// droits, mais ne transfère pas le sien.
	CanAssignGroupHead bool
}

func (h *PagesHandler) AmapAdminRightsAddPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "rights")
	if !ok {
		return
	}

	data := AmapAdminRightsAddData{AmapAdminPageData: base}
	data.Title = "Ajouter un droit"
	data.CanAssignGroupHead = base.IsTechnicalManager

	h.db.Where("group_id = ?", base.Group.ID).Preload("User").Find(&data.Members)
	// Le responsable technique a tous les droits par construction (cf.
	// loadGroupAccess) : il ne doit pas apparaître parmi les cibles modifiables.
	filtered := data.Members[:0]
	for _, ug := range data.Members {
		if !isTechnicalManagerEmail(ug.User.Email) {
			filtered = append(filtered, ug)
		}
	}
	data.Members = filtered
	h.db.Where("group_id = ?", base.Group.ID).Find(&data.Catalogs)

	if h := exclusiveHolder(h.db, base.Group.ID, 0, model.RightGroupAdmin); h != nil {
		data.GroupAdminHolder = h.User.Name()
	}

	if c.Request.Method == http.MethodPost {
		userIDStr := c.PostForm("user_id")
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil || userID == 0 {
			data.Error = "Veuillez sélectionner un membre."
			renderRightsAdd(c, data)
			return
		}

		if isTechnicalManager(h.db, uint(userID)) {
			data.Error = "Le responsable technique tient son rôle de la configuration : ses droits ne se modifient pas ici."
			renderRightsAdd(c, data)
			return
		}

		var ug model.UserGroup
		if err := h.db.Where("user_id = ? AND group_id = ?", userID, base.Group.ID).First(&ug).Error; err != nil {
			data.Error = "Membre introuvable."
			renderRightsAdd(c, data)
			return
		}

		rights := ug.GetRights()

		addRight := func(r model.Right, params ...string) {
			for _, existing := range rights {
				if existing.Right == r {
					if len(params) == 0 {
						return
					}
					for _, p := range existing.Params {
						for _, want := range params {
							if p == want {
								return
							}
						}
					}
					// ajouter le param à l'entrée existante
					for i, existing2 := range rights {
						if existing2.Right == r {
							rights[i].Params = append(rights[i].Params, params...)
							return
						}
					}
				}
			}
			rights = append(rights, model.UserRight{Right: r, Params: func() []string {
				if len(params) == 0 {
					return nil
				}
				return params
			}()})
		}

		// Le rôle de responsable ne se donne que par le responsable technique :
		// c'est lui qui l'attribue à l'ouverture d'un groupe, et lui seul qui
		// le déplace ensuite. Le contrôle est refait ici, le formulaire ne
		// masquant la case que côté affichage.
		if data.CanAssignGroupHead && c.PostForm("right_group_admin") != "" {
			addRight(model.RightGroupAdmin)
		}
		if c.PostForm("right_membership") != "" {
			addRight(model.RightMembership)
		}
		if c.PostForm("right_messages") != "" {
			addRight(model.RightMessages)
		}
		if c.PostForm("right_distributions") != "" {
			addRight(model.RightDistributions)
		}
		if c.PostForm("right_parameters") != "" {
			addRight(model.RightParameters)
		}
		if c.PostForm("catalog_all") != "" {
			addRight(model.RightCatalogAdmin)
		} else {
			for _, cat := range data.Catalogs {
				if c.PostForm(fmt.Sprintf("catalog_%d", cat.ID)) != "" {
					addRight(model.RightCatalogAdmin, strconv.FormatUint(uint64(cat.ID), 10))
				}
			}
		}

		// Responsable de groupe et responsable technique n'ont qu'un titulaire :
		// les accorder ici les retire à qui les détenait.
		transfers, err := transferExclusiveRights(h.db, base.Group.ID, uint(userID), rights)
		if err != nil {
			data.Error = "Transfert impossible : " + err.Error()
			renderRightsAdd(c, data)
			return
		}

		import_json, _ := json.Marshal(rights)
		ug.Rights = string(import_json)
		// Update ciblé sur la colonne, et non Save de la structure : Save
		// réécrit aussi l'utilisateur associé chargé par Preload, dont la
		// birth_date vaut « 0000-00-00 » pour 93 comptes issus de l'import.
		// MySQL en mode strict refuse cette date, et tout l'enregistrement
		// échouait — sans un mot, l'erreur n'étant pas inspectée.
		if err := h.db.Model(&model.UserGroup{}).
			Where("user_id = ? AND group_id = ?", ug.UserID, ug.GroupID).
			Update("rights", ug.Rights).Error; err != nil {
			data.Error = "Enregistrement impossible : " + err.Error()
			renderRightsAdd(c, data)
			return
		}
		c.Redirect(http.StatusFound, "/amapadmin/rights"+transferQuery(transfers))
		return
	}

	renderRightsAdd(c, data)
}

func renderRightsAdd(c *gin.Context, data AmapAdminRightsAddData) {
	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_rights_add.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET+POST /amapadmin/rights/edit/:userId ----

type AmapAdminRightsEditData struct {
	AmapAdminPageData
	Member           model.UserGroup
	Catalogs         []model.Catalog
	HasGroupAdmin    bool
	HasMembership    bool
	HasMessages      bool
	HasDistributions bool
	HasParameters    bool
	HasAllCatalogs   bool
	CatalogRights    map[string]bool
	Error            string

	// Titulaire actuel du rôle à titulaire unique, hors membre édité.
	GroupAdminHolder string

	// CanAssignGroupHead : voir AmapAdminRightsAddData.
	CanAssignGroupHead bool
}

func (h *PagesHandler) AmapAdminRightsEditPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "rights")
	if !ok {
		return
	}

	userID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	// Les droits du responsable technique ne sont pas modifiables : il les a
	// tous par construction (cf. handler.loadGroupAccess), et son rôle se
	// transfère en changeant la configuration, pas depuis cet écran.
	if isTechnicalManager(h.db, uint(userID)) {
		c.String(http.StatusForbidden, "le responsable technique tient son rôle de la configuration : ses droits ne se modifient pas ici")
		return
	}

	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", userID, base.Group.ID).Preload("User").First(&ug).Error; err != nil {
		c.String(http.StatusNotFound, "membre introuvable")
		return
	}

	var catalogs []model.Catalog
	h.db.Where("group_id = ?", base.Group.ID).Find(&catalogs)

	data := AmapAdminRightsEditData{
		AmapAdminPageData: base,
		Member:            ug,
		Catalogs:          catalogs,
		CatalogRights:     make(map[string]bool),
	}
	data.Title = "Modifier les droits"
	data.CanAssignGroupHead = base.IsTechnicalManager

	if holder := exclusiveHolder(h.db, base.Group.ID, uint(userID), model.RightGroupAdmin); holder != nil {
		data.GroupAdminHolder = holder.User.Name()
	}

	fillRightsState := func(rights []model.UserRight) {
		for _, r := range rights {
			switch r.Right {
			case model.RightGroupAdmin:
				data.HasGroupAdmin = true
			case model.RightMembership:
				data.HasMembership = true
			case model.RightMessages:
				data.HasMessages = true
			case model.RightDistributions:
				data.HasDistributions = true
			case model.RightParameters:
				data.HasParameters = true
			case model.RightCatalogAdmin:
				if len(r.Params) == 0 {
					data.HasAllCatalogs = true
				} else {
					for _, p := range r.Params {
						data.CatalogRights[p] = true
					}
				}
			}
		}
	}

	if c.Request.Method == http.MethodPost {
		var rights []model.UserRight
		switch {
		case data.CanAssignGroupHead:
			if c.PostForm("right_group_admin") != "" {
				rights = append(rights, model.UserRight{Right: model.RightGroupAdmin})
			}
		case ug.IsGroupHead():
			// Le formulaire ne montre pas la case à qui ne peut pas l'accorder,
			// donc le POST ne la porte pas : reconstruire la liste sans elle
			// retirerait son rôle au responsable dès qu'on touche à ses autres
			// droits. On le reconduit explicitement.
			rights = append(rights, model.UserRight{Right: model.RightGroupAdmin})
		}
		if c.PostForm("right_membership") != "" {
			rights = append(rights, model.UserRight{Right: model.RightMembership})
		}
		if c.PostForm("right_messages") != "" {
			rights = append(rights, model.UserRight{Right: model.RightMessages})
		}
		if c.PostForm("right_distributions") != "" {
			rights = append(rights, model.UserRight{Right: model.RightDistributions})
		}
		if c.PostForm("right_parameters") != "" {
			rights = append(rights, model.UserRight{Right: model.RightParameters})
		}
		if c.PostForm("catalog_all") != "" {
			rights = append(rights, model.UserRight{Right: model.RightCatalogAdmin})
		} else {
			var catParams []string
			for _, cat := range catalogs {
				key := strconv.FormatUint(uint64(cat.ID), 10)
				if c.PostForm("catalog_"+key) != "" {
					catParams = append(catParams, key)
				}
			}
			if len(catParams) > 0 {
				rights = append(rights, model.UserRight{Right: model.RightCatalogAdmin, Params: catParams})
			}
		}

		// Le groupe ne doit pas se retrouver sans responsable : le seul
		// titulaire ne peut pas se retirer le rôle sans le passer à quelqu'un.
		// Le responsable technique resterait un recours, mais plus aucun membre ne
		// pourrait administrer le groupe.
		if leavesGroupWithoutManager(h.db, base.Group.ID, uint(userID), rights) {
			data.Error = "Ce membre est le seul responsable du groupe. Désignez d'abord quelqu'un d'autre."
			fillRightsState(ug.GetRights())
			renderRightsEdit(c, data)
			return
		}

		transfers, err := transferExclusiveRights(h.db, base.Group.ID, uint(userID), rights)
		if err != nil {
			data.Error = "Transfert impossible : " + err.Error()
			fillRightsState(ug.GetRights())
			renderRightsEdit(c, data)
			return
		}

		encoded, _ := json.Marshal(rights)
		ug.Rights = string(encoded)
		// Update ciblé sur la colonne, et non Save de la structure : Save
		// réécrit aussi l'utilisateur associé chargé par Preload, dont la
		// birth_date vaut « 0000-00-00 » pour 93 comptes issus de l'import.
		// MySQL en mode strict refuse cette date, et tout l'enregistrement
		// échouait — sans un mot, l'erreur n'étant pas inspectée.
		if err := h.db.Model(&model.UserGroup{}).
			Where("user_id = ? AND group_id = ?", ug.UserID, ug.GroupID).
			Update("rights", ug.Rights).Error; err != nil {
			data.Error = "Enregistrement impossible : " + err.Error()
			fillRightsState(ug.GetRights())
			renderRightsEdit(c, data)
			return
		}
		c.Redirect(http.StatusFound, "/amapadmin/rights"+transferQuery(transfers))
		return
	}

	fillRightsState(ug.GetRights())
	renderRightsEdit(c, data)
}

func renderRightsEdit(c *gin.Context, data AmapAdminRightsEditData) {
	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_rights_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// transferQuery construit le fragment d'URL annonçant les rôles repris à leur
// titulaire précédent. Sans lui, la dépossession serait muette.
func transferQuery(transfers map[model.Right]string) string {
	if len(transfers) == 0 {
		return ""
	}
	parts := make([]string, 0, len(transfers))
	for _, r := range model.ExclusiveRights() {
		if who, ok := transfers[r]; ok {
			parts = append(parts, r.Label()+" retiré à "+who)
		}
	}
	return "?transfert=" + url.QueryEscape(strings.Join(parts, " ; "))
}

// ---- AmapAdmin shared page data ----

type AmapAdminPageData struct {
	PageData
	AmapAdminTab string
	// AmapAdminTitre et AmapAdminChapeau : le titre de l'onglet ouvert et sa
	// phrase d'explication. Portés par la coquille commune plutôt que répétés
	// dans chaque gabarit — la mise en page de l'en-tête ne s'écrit ainsi
	// qu'une fois.
	AmapAdminTitre   string
	AmapAdminChapeau string
}

// amapAdminEntete : le titre et la phrase d'explication de chaque onglet des
// paramètres. Une seule table, lue par la coquille et par le fil d'Ariane :
// deux libellés tenus à la main auraient fini par se contredire.
func amapAdminEntete(tab string) (titre, chapeau string, ok bool) {
	switch tab {
	case "rights":
		return "Droits d'administration",
			"Qui peut faire quoi dans le groupe. Chaque délégation ouvre un domaine, et lui seul.", true
	case "vatRates":
		return "Taux de TVA",
			"Les taux proposés à la saisie d'un produit.", true
	case "volunteers":
		return "Permanences",
			"Les postes que les adhérents peuvent tenir lors d'une distribution.", true
	case "membership":
		return "Adhésions",
			"Le montant et le rythme de l'adhésion annuelle au groupe.", true
	case "currency":
		return "Monnaie",
			"L'unité dans laquelle s'affichent les prix et les soldes.", true
	case "documents":
		return "Documents",
			"Les fichiers mis à disposition des adhérents — statuts, charte, règlement.", true
	}
	return "", "", false
}

func (h *PagesHandler) buildAmapAdminData(c *gin.Context, tab string) (AmapAdminPageData, bool) {
	pd := h.buildPageData(c)
	// La délégation « paramètres » suffit : elle a été taillée pour ces écrans.
	// L'attribution des droits, elle, reste gardée par RequireRightsManagement.
	if pd.User == nil || pd.Group == nil || !pd.HasParameters {
		c.Redirect(http.StatusFound, "/home")
		return AmapAdminPageData{}, false
	}
	pd.Category = "amapadmin"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}
	data := AmapAdminPageData{PageData: pd, AmapAdminTab: tab}
	data.Category = "amapadmin"
	// Même largeur que les autres écrans de gestion.
	data.Container = "container-fluid ac-accueil"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}
	if titre, chapeau, ok := amapAdminEntete(tab); ok {
		data.AmapAdminTitre, data.AmapAdminChapeau = titre, chapeau
		data.Breadcrumb = append(data.Breadcrumb, BreadcrumbItem{Name: titre})
	}
	return data, true
}

// ---- GET /amapadmin/vatRates ----

type VatEntry struct {
	Slot int
	Name string
	Rate float64
}

type VatRatesData struct {
	AmapAdminPageData
	Vats     []VatEntry
	FreeSlot int // 0 si plus de slot libre
}

func (h *PagesHandler) AmapAdminVatRatesPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "vatRates")
	if !ok {
		return
	}
	data := VatRatesData{AmapAdminPageData: base}
	data.Title = "Taux de TVA"
	g := base.Group
	names := [4]string{g.VatName1, g.VatName2, g.VatName3, g.VatName4}
	rates := [4]float64{g.VatRate1, g.VatRate2, g.VatRate3, g.VatRate4}
	for i, n := range names {
		if strings.TrimSpace(n) != "" {
			data.Vats = append(data.Vats, VatEntry{Slot: i + 1, Name: n, Rate: rates[i]})
		} else if data.FreeSlot == 0 {
			data.FreeSlot = i + 1
		}
	}

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_vatrates.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (h *PagesHandler) AmapAdminVatRatesUpdate(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	if c.PostForm("action") == "delete" {
		slot, _ := strconv.Atoi(c.PostForm("slot"))
		if slot >= 1 && slot <= 4 {
			h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(map[string]interface{}{
				fmt.Sprintf("vat_name%d", slot): "",
				fmt.Sprintf("vat_rate%d", slot): 0,
			})
		}
		c.Redirect(http.StatusFound, "/amapadmin/vatRates")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.Redirect(http.StatusFound, "/amapadmin/vatRates")
		return
	}
	var rate float64
	fmt.Sscanf(c.PostForm("rate"), "%f", &rate)

	var g model.Group
	h.db.First(&g, pd.Group.ID)
	slots := []struct {
		name string
		rate float64
	}{
		{g.VatName1, g.VatRate1},
		{g.VatName2, g.VatRate2},
		{g.VatName3, g.VatRate3},
		{g.VatName4, g.VatRate4},
	}
	for i, s := range slots {
		if strings.TrimSpace(s.name) == "" {
			h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(map[string]interface{}{
				fmt.Sprintf("vat_name%d", i+1): name,
				fmt.Sprintf("vat_rate%d", i+1): rate,
			})
			break
		}
	}
	c.Redirect(http.StatusFound, "/amapadmin/vatRates")
}

// ---- GET /amapadmin/volunteers ----

type AmapAdminVolunteersData struct {
	AmapAdminPageData
	VolunteerRoles []model.VolunteerRole
}

func (h *PagesHandler) AmapAdminVolunteersPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "volunteers")
	if !ok {
		return
	}
	data := AmapAdminVolunteersData{AmapAdminPageData: base}
	data.Title = "Permanences"
	h.db.Where("group_id = ?", base.Group.ID).Preload("Catalog").Find(&data.VolunteerRoles)

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_volunteers.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET /amapadmin/membership ----

func (h *PagesHandler) AmapAdminMembershipPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "membership")
	if !ok {
		return
	}
	base.Title = "Adhésions"

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_membership.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", base); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (h *PagesHandler) AmapAdminMembershipUpdate(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	updates := map[string]interface{}{
		"has_membership": c.PostForm("hasMembership") == "1",
	}
	if fee := strings.TrimSpace(c.PostForm("membershipFee")); fee != "" {
		if n, err := strconv.Atoi(fee); err == nil {
			updates["membership_fee"] = n
		}
	} else {
		updates["membership_fee"] = nil
	}
	if d := strings.TrimSpace(c.PostForm("membershipRenewalDate")); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			updates["membership_renewal_date"] = t
		}
	} else {
		updates["membership_renewal_date"] = nil
	}
	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(updates)
	c.Redirect(http.StatusFound, "/amapadmin/membership")
}

// ---- GET /amapadmin/currency ----

func (h *PagesHandler) AmapAdminCurrencyPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "currency")
	if !ok {
		return
	}
	base.Title = "Monnaie"

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_currency.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", base); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (h *PagesHandler) AmapAdminCurrencyUpdate(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(map[string]interface{}{
		"currency":      c.PostForm("currency"),
		"currency_code": c.PostForm("currencyCode"),
	})
	c.Redirect(http.StatusFound, "/amapadmin/currency")
}

// ---- GET /amapadmin/documents ----

type DocView struct {
	ID        uint
	Name      string
	FileName  string
	URL       string
	CreatedAt string
	SizeLabel string
}

type AmapAdminDocumentsData struct {
	AmapAdminPageData
	Docs     []DocView
	ErrorMsg string
}

func (h *PagesHandler) AmapAdminDocumentsPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "documents")
	if !ok {
		return
	}
	data := AmapAdminDocumentsData{AmapAdminPageData: base}
	data.Title = "Documents"

	switch c.Query("err") {
	case "nofile":
		data.ErrorMsg = "Veuillez choisir un fichier."
	case "notpdf":
		data.ErrorMsg = "Seuls les fichiers PDF sont acceptés."
	case "toobig":
		data.ErrorMsg = "Le fichier dépasse la taille maximale (10 Mo)."
	}

	var docs []model.GroupDoc
	h.db.Where("group_id = ?", base.Group.ID).Preload("File").
		Order("created_at DESC").Find(&docs)
	for _, d := range docs {
		size := len(d.File.Data)
		var sizeLabel string
		if size >= 1024*1024 {
			sizeLabel = fmt.Sprintf("%.1f Mo", float64(size)/(1024*1024))
		} else {
			sizeLabel = fmt.Sprintf("%d Ko", size/1024)
		}
		data.Docs = append(data.Docs, DocView{
			ID:        d.ID,
			Name:      d.Name,
			FileName:  d.File.Name,
			URL:       FileURL(d.File.ID, h.cfg.Key, d.File.Name),
			CreatedAt: d.CreatedAt.Format("02/01/2006"),
			SizeLabel: sizeLabel,
		})
	}

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_documents.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- POST /amapadmin/documents (upload) ----

const maxDocSize = 10 * 1024 * 1024 // 10 MB

func (h *PagesHandler) AmapAdminDocumentsUpload(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		c.Redirect(http.StatusFound, "/amapadmin/documents?err=nofile")
		return
	}
	if !strings.HasSuffix(strings.ToLower(fh.Filename), ".pdf") {
		c.Redirect(http.StatusFound, "/amapadmin/documents?err=notpdf")
		return
	}
	if fh.Size > maxDocSize {
		c.Redirect(http.StatusFound, "/amapadmin/documents?err=toobig")
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}

	file := model.File{Name: fh.Filename, Data: data}
	if err := h.db.Create(&file).Error; err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = fh.Filename
	}
	doc := model.GroupDoc{GroupID: pd.Group.ID, FileID: file.ID, Name: name}
	h.db.Create(&doc)
	c.Redirect(http.StatusFound, "/amapadmin/documents")
}

// ---- GET /amapadmin/documents/delete/:id ----

func (h *PagesHandler) AmapAdminDocumentsDelete(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var doc model.GroupDoc
	if err := h.db.Where("id = ? AND group_id = ?", id, pd.Group.ID).First(&doc).Error; err != nil {
		c.Redirect(http.StatusFound, "/amapadmin/documents")
		return
	}
	fileID := doc.FileID
	h.db.Delete(&doc)
	h.db.Delete(&model.File{}, fileID)
	c.Redirect(http.StatusFound, "/amapadmin/documents")
}

// ---- POST /amapadmin/logo ----

const maxLogoSize = 5 * 1024 * 1024 // 5 MB

func (h *PagesHandler) AmapAdminLogoUpload(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	fh, err := c.FormFile("logo")
	if err != nil || fh == nil {
		c.Redirect(http.StatusFound, "/amapadmin")
		return
	}
	name := strings.ToLower(fh.Filename)
	allowed := false
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		if strings.HasSuffix(name, ext) {
			allowed = true
			break
		}
	}
	if !allowed || fh.Size > maxLogoSize {
		c.Redirect(http.StatusFound, "/amapadmin")
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}

	// Supprimer l'ancien logo s'il existe
	var current model.Group
	h.db.First(&current, pd.Group.ID)
	if current.LogoID != nil {
		oldID := *current.LogoID
		h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Update("logoId", nil)
		h.db.Delete(&model.File{}, oldID)
	}

	file := model.File{Name: fh.Filename, Data: data}
	if err := h.db.Create(&file).Error; err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}
	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Update("logoId", file.ID)
	c.Redirect(http.StatusFound, "/amapadmin")
}

// ---- GET /amapadmin/logo/delete ----

func (h *PagesHandler) AmapAdminLogoDelete(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	var current model.Group
	h.db.First(&current, pd.Group.ID)
	if current.LogoID != nil {
		oldID := *current.LogoID
		h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Update("logoId", nil)
		h.db.Delete(&model.File{}, oldID)
	}
	c.Redirect(http.StatusFound, "/amapadmin")
}

// ---- GET /group/:id — public group page ----

type GroupPublicDistrib struct {
	DayOfWeek string
	Day       string
	Month     string
	Place     string
	Address   string
	Hours     string
	Active    bool
}

type GroupPublicProduct struct {
	Name string
	URL  string
}

type GroupPublicVendor struct {
	Name     string
	Address  string
	Organic  bool
	Products []GroupPublicProduct
}

type GroupPublicDocView struct {
	Name string
	URL  string
}

type GroupPublicData struct {
	Title        string
	Group        *model.Group
	LogoURL      string
	Intro        string
	Home         string
	ExtURL       string
	ContactName  string
	ContactEmail string
	ContactPhone string
	ShowPhone    bool
	Distribs     []GroupPublicDistrib
	Vendors      []GroupPublicVendor
	Documents    []GroupPublicDocView
	LoggedIn     bool
	Container    string
}

func (h *PagesHandler) GroupPublicPage(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusNotFound, "groupe introuvable")
		return
	}
	var g model.Group
	if err := h.db.Preload("Contact").Preload("Logo").First(&g, groupID).Error; err != nil {
		c.String(http.StatusNotFound, "groupe introuvable")
		return
	}

	claims := middleware.GetClaims(c)
	data := GroupPublicData{
		Title:     g.Name,
		Group:     &g,
		Container: "container-fluid",
		LoggedIn:  claims != nil,
	}
	if g.Logo != nil {
		data.LogoURL = FileURL(g.Logo.ID, h.cfg.Key, g.Logo.Name)
	}
	if g.TxtIntro != nil {
		data.Intro = *g.TxtIntro
	}
	if g.TxtHome != nil {
		data.Home = *g.TxtHome
	}
	if g.ExtURL != nil {
		data.ExtURL = *g.ExtURL
	}
	if g.Contact != nil {
		data.ContactName = g.Contact.FirstName + " " + g.Contact.LastName
		data.ContactEmail = g.Contact.Email
		if g.Contact.Phone != nil {
			data.ContactPhone = *g.Contact.Phone
		}
		data.ShowPhone = g.CanExposePhone() && data.ContactPhone != ""
	}

	now := time.Now()
	frMonths := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	frDaysFull := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}

	var mds []model.MultiDistrib
	h.db.Where("group_id = ? AND distrib_end_date >= ?", g.ID, now).
		Preload("Place").Order("distrib_start_date ASC").Limit(5).Find(&mds)
	for _, md := range mds {
		s := md.DistribStartDate
		e := md.DistribEndDate
		addr := ""
		if md.Place.Address != nil {
			addr = *md.Place.Address
		}
		if md.Place.City != nil {
			if addr != "" {
				addr += ", "
			}
			addr += *md.Place.City
		}
		isToday := s.Year() == now.Year() && s.Month() == now.Month() && s.Day() == now.Day()
		data.Distribs = append(data.Distribs, GroupPublicDistrib{
			DayOfWeek: frDaysFull[s.Weekday()],
			Day:       fmt.Sprintf("%d", s.Day()),
			Month:     frMonths[s.Month()],
			Place:     md.Place.Name,
			Address:   addr,
			Hours:     fmt.Sprintf("%02d:%02d – %02d:%02d", s.Hour(), s.Minute(), e.Hour(), e.Minute()),
			Active:    isToday,
		})
	}

	var cats []model.Catalog
	h.db.Where("group_id = ? AND (end_date IS NULL OR end_date > ?) AND (start_date IS NULL OR start_date <= ?)",
		g.ID, now, now).
		Preload("Vendor").Find(&cats)
	seen := map[uint]int{}
	for _, cat := range cats {
		idx, ok := seen[cat.VendorID]
		if !ok {
			addr := ""
			if cat.Vendor.ZipCode != nil {
				addr = *cat.Vendor.ZipCode
			}
			if cat.Vendor.City != nil {
				if addr != "" {
					addr += " "
				}
				addr += *cat.Vendor.City
			}
			data.Vendors = append(data.Vendors, GroupPublicVendor{
				Name:    cat.Vendor.Name,
				Address: addr,
				Organic: cat.Vendor.Organic,
			})
			idx = len(data.Vendors) - 1
			seen[cat.VendorID] = idx
		}
		if len(data.Vendors[idx].Products) >= 4 {
			continue
		}
		remaining := 4 - len(data.Vendors[idx].Products)
		var prods []model.Product
		h.db.Where("catalog_id = ? AND active = ?", cat.ID, true).
			Preload("Image").Limit(remaining).Find(&prods)
		for _, p := range prods {
			url := ""
			if p.Image != nil {
				url = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
			}
			data.Vendors[idx].Products = append(data.Vendors[idx].Products, GroupPublicProduct{
				Name: p.Name, URL: url,
			})
		}
	}

	var docs []model.GroupDoc
	h.db.Where("group_id = ?", g.ID).Preload("File").Order("created_at DESC").Find(&docs)
	for _, d := range docs {
		data.Documents = append(data.Documents, GroupPublicDocView{
			Name: d.Name,
			URL:  FileURL(d.File.ID, h.cfg.Key, d.File.Name),
		})
	}

	t, err := loadTemplates("base.html", "group_public.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Postes de bénévole : création, modification, suppression ----
//
// L'écran des permanences listait les postes et offrait trois liens — nouveau,
// modifier, supprimer — dont aucun n'était servi : les trois répondaient 404.

type VolunteerRoleFormData struct {
	AmapAdminPageData
	// Action : l'adresse du formulaire. Le même gabarit sert à créer et à
	// modifier ; seule cette adresse les distingue.
	Action    string
	IsNew     bool
	Name      string
	CatalogID uint
	Catalogs  []model.Catalog
	Error     string
}

// volunteerRoleForm sert la création comme la modification. Le poste vaut nil
// à la création.
func (h *PagesHandler) volunteerRoleForm(c *gin.Context, role *model.VolunteerRole) {
	base, ok := h.buildAmapAdminData(c, "volunteers")
	if !ok {
		return
	}

	data := VolunteerRoleFormData{AmapAdminPageData: base, IsNew: role == nil}
	if role == nil {
		data.Action = "/amapadmin/volunteers/new"
		data.Title = "Nouveau poste"
		data.AmapAdminTitre = "Nouveau poste"
	} else {
		data.Action = fmt.Sprintf("/amapadmin/volunteers/edit/%d", role.ID)
		data.Title = "Modifier un poste"
		data.AmapAdminTitre = "Modifier un poste"
		data.Name = role.Name
		if role.CatalogID != nil {
			data.CatalogID = *role.CatalogID
		}
	}
	data.AmapAdminChapeau = "Ce qu'il y a à tenir pendant une distribution, et " +
		"les jours où le poste est proposé."
	data.Breadcrumb = []BreadcrumbItem{
		{Name: "Paramètres", Link: "/amapadmin"},
		{Name: "Permanences", Link: "/amapadmin/volunteers"},
		{Name: data.AmapAdminTitre},
	}

	// Les catalogues du groupe, pour rattacher le poste à un producteur.
	h.db.Where("group_id = ?", base.Group.ID).Order("name ASC").Find(&data.Catalogs)

	if c.Request.Method == http.MethodPost {
		data.Name = strings.TrimSpace(c.PostForm("name"))
		var catalogID *uint
		if v := strings.TrimSpace(c.PostForm("catalog_id")); v != "" {
			if id, err := strconv.ParseUint(v, 10, 64); err == nil && id > 0 {
				// Le catalogue doit être de ce groupe : sans cette vérification,
				// un identifiant posté à la main rattacherait le poste au
				// catalogue d'une autre AMAP.
				for _, cat := range data.Catalogs {
					if cat.ID == uint(id) {
						cid := cat.ID
						catalogID = &cid
						break
					}
				}
				if catalogID == nil {
					data.Error = "Ce catalogue n'appartient pas au groupe."
				}
			}
		}
		if data.Error == "" && data.Name == "" {
			data.Error = "Le poste a besoin d'un nom."
		}
		if data.Error == "" && utf8.RuneCountInString(data.Name) > 128 {
			data.Error = "Le nom du poste ne peut pas dépasser 128 caractères."
		}

		if data.Error == "" {
			if role == nil {
				h.db.Create(&model.VolunteerRole{
					GroupID:   base.Group.ID,
					Name:      data.Name,
					CatalogID: catalogID,
				})
			} else {
				h.db.Model(&model.VolunteerRole{}).Where("id = ?", role.ID).
					Updates(map[string]interface{}{
						"name":       data.Name,
						"catalog_id": catalogID,
					})
			}
			c.Redirect(http.StatusFound, "/amapadmin/volunteers")
			return
		}
		if catalogID != nil {
			data.CatalogID = *catalogID
		}
	}

	t, err := loadTemplates("base.html", "design.html", "cycles_style.html",
		"amapadmin_layout.html", "amapadmin_volunteer_form.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET|POST /amapadmin/volunteers/new ----

func (h *PagesHandler) AmapAdminVolunteerNewPage(c *gin.Context) {
	h.volunteerRoleForm(c, nil)
}

// roleDuGroupe retrouve un poste en le bornant au groupe courant : un
// identifiant seul désignerait aussi bien le poste d'une autre AMAP.
func (h *PagesHandler) roleDuGroupe(c *gin.Context, groupID uint) (*model.VolunteerRole, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/amapadmin/volunteers")
		return nil, false
	}
	var role model.VolunteerRole
	if err := h.db.Where("id = ? AND group_id = ?", uint(id), groupID).
		First(&role).Error; err != nil {
		c.Redirect(http.StatusFound, "/amapadmin/volunteers")
		return nil, false
	}
	return &role, true
}

// ---- GET|POST /amapadmin/volunteers/edit/:id ----

func (h *PagesHandler) AmapAdminVolunteerEditPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasParameters {
		c.Redirect(http.StatusFound, "/home")
		return
	}
	role, ok := h.roleDuGroupe(c, pd.Group.ID)
	if !ok {
		return
	}
	h.volunteerRoleForm(c, role)
}

// ---- POST /amapadmin/volunteers/delete/:id ----
//
// En POST : supprimer un poste efface aussi les inscriptions qui s'y
// rattachent, et cela ne doit pas pouvoir arriver au simple chargement d'une
// adresse.

func (h *PagesHandler) AmapAdminVolunteerDelete(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasParameters {
		c.Redirect(http.StatusFound, "/home")
		return
	}
	role, ok := h.roleDuGroupe(c, pd.Group.ID)
	if !ok {
		return
	}
	// Les inscriptions partent avec le poste : laissées derrière, elles
	// désigneraient un rôle disparu sur les listes d'émargement.
	h.db.Where("volunteer_role_id = ?", role.ID).Delete(&model.VolunteerRoleAssignment{})
	h.db.Delete(&model.VolunteerRole{}, role.ID)
	c.Redirect(http.StatusFound, "/amapadmin/volunteers")
}
