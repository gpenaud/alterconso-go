package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// ---- /account/quit ----

func (h *PagesHandler) AccountQuitPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	if c.Query("token") != "" {
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

	t, err := loadTemplates("base.html", "design.html", "account_quit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /member/waiting ----

type WaitingListData struct {
	PageData
	WaitingList []WaitingEntry
}

type WaitingEntry struct {
	UserID   uint
	Name     string
	Email    string
	Phone    string
	Date     string
	Message  string
}

func (h *PagesHandler) MemberWaitingPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	// Accept membership
	if addID := c.Query("add"); addID != "" {
		uid, _ := strconv.ParseUint(addID, 10, 64)
		if uid != 0 {
			// Remove from waiting list
			h.db.Where("user_id = ? AND catalog_id IN (SELECT id FROM catalogs WHERE group_id = ?)",
				uid, pd.Group.ID).Delete(&model.WaitingList{})
			// Add to group if not already
			var existing model.UserGroup
			if h.db.Where("user_id = ? AND group_id = ?", uid, pd.Group.ID).First(&existing).Error != nil {
				h.db.Create(&model.UserGroup{UserID: uint(uid), GroupID: pd.Group.ID})
			}
		}
		c.Redirect(http.StatusFound, "/member/waiting")
		return
	}

	// Deny request
	if removeID := c.Query("remove"); removeID != "" {
		uid, _ := strconv.ParseUint(removeID, 10, 64)
		if uid != 0 {
			h.db.Where("user_id = ? AND catalog_id IN (SELECT id FROM catalogs WHERE group_id = ?)",
				uid, pd.Group.ID).Delete(&model.WaitingList{})
		}
		c.Redirect(http.StatusFound, "/member/waiting")
		return
	}

	// Load waiting list for this group's catalogs
	var wl []model.WaitingList
	h.db.Joins("JOIN catalogs ON catalogs.id = waiting_lists.catalog_id").
		Where("catalogs.group_id = ?", pd.Group.ID).
		Preload("User").
		Order("waiting_lists.created_at ASC").
		Find(&wl)

	data := WaitingListData{PageData: pd}
	data.Title = "Liste d'attente"
	data.Category = "member"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Membres", Link: "/member"}}
	for _, w := range wl {
		phone := ""
		if w.User.Phone != nil {
			phone = *w.User.Phone
		}
		msg := ""
		if w.Message != nil {
			msg = *w.Message
		}
		data.WaitingList = append(data.WaitingList, WaitingEntry{
			UserID:  w.UserID,
			Name:    w.User.FirstName + " " + w.User.LastName,
			Email:   w.User.Email,
			Phone:   phone,
			Date:    w.CreatedAt.Format("02/01/2006"),
			Message: msg,
		})
	}

	t, err := loadTemplates("base.html", "design.html", "member_waiting.html")
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
			SmartQty:    formatQty(o.Quantity, o.Product.UnitType),
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
		// Delete existing volunteer records for this multidistrib
		h.db.Where("multi_distrib_id = ?", md.ID).Delete(&model.Volunteer{})
		// Re-create from form
		for _, r := range roles {
			key := "role_" + strconv.Itoa(int(r.ID))
			userIDStr := c.PostForm(key)
			if userIDStr == "" || userIDStr == "0" {
				continue
			}
			uid, err := strconv.ParseUint(userIDStr, 10, 64)
			if err != nil || uid == 0 {
				continue
			}
			roleName := r.Name
			h.db.Create(&model.Volunteer{
				UserID:         uint(uid),
				MultiDistribID: md.ID,
				Role:           &roleName,
			})
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

	t, err := loadTemplates("base.html", "design.html", "distribution_volunteers_summary.html")
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
}

type VolParticipationRow struct {
	UserID   uint
	Name     string
	Done     int
	ToBeDone int
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

	t, err := loadTemplates("base.html", "design.html", "distribution_volunteers_participation.html")
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
}

type RightUserView struct {
	UserID       uint
	Name         string
	Rights       []string
	IsSuperAdmin bool
}

func formatRightLabels(rights []model.UserRight, catalogMap map[string]string) []string {
	// Si l'utilisateur est administrateur de groupe, les autres droits sont implicites.
	for _, r := range rights {
		if r.Right == model.RightGroupAdmin {
			return []string{"Administrateur de groupe"}
		}
	}
	var labels []string
	for _, r := range rights {
		switch r.Right {
		case model.RightMembership:
			labels = append(labels, "Gestion des membres")
		case model.RightMessages:
			labels = append(labels, "Messages")
		case model.RightDatabaseAdmin:
			labels = append(labels, "Gestion de la base de données")
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

	var ugs []model.UserGroup
	h.db.Where("group_id = ?", base.Group.ID).Preload("User").Find(&ugs)

	var catalogs []model.Catalog
	h.db.Where("group_id = ?", base.Group.ID).Find(&catalogs)
	catalogMap := make(map[string]string, len(catalogs))
	for _, cat := range catalogs {
		catalogMap[strconv.FormatUint(uint64(cat.ID), 10)] = cat.Name
	}

	data := AmapAdminRightsData{AmapAdminPageData: base}
	data.Title = "Droits d'administration"

	for _, ug := range ugs {
		rights := ug.GetRights()
		isSA := ug.User.IsAdmin()
		// Le superadmin global est toujours listé avec tous les droits, même
		// s'il n'a pas de UserGroup.Rights persisté (cf. loadGroupAccess).
		if len(rights) == 0 && !isSA {
			continue
		}
		labels := formatRightLabels(rights, catalogMap)
		if isSA && len(labels) == 0 {
			labels = []string{"Administrateur de groupe"}
		}
		rv := RightUserView{
			UserID:       ug.UserID,
			Name:         ug.User.FirstName + " " + ug.User.LastName,
			Rights:       labels,
			IsSuperAdmin: isSA,
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
}

func (h *PagesHandler) AmapAdminRightsAddPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "rights")
	if !ok {
		return
	}

	data := AmapAdminRightsAddData{AmapAdminPageData: base}
	data.Title = "Ajouter un droit"

	h.db.Where("group_id = ?", base.Group.ID).Preload("User").Find(&data.Members)
	// Le superadmin global a tous les droits par construction (cf. loadGroupAccess) :
	// il ne doit pas apparaître dans la liste des cibles modifiables.
	filtered := data.Members[:0]
	for _, ug := range data.Members {
		if !ug.User.IsAdmin() {
			filtered = append(filtered, ug)
		}
	}
	data.Members = filtered
	h.db.Where("group_id = ?", base.Group.ID).Find(&data.Catalogs)

	if c.Request.Method == http.MethodPost {
		userIDStr := c.PostForm("user_id")
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil || userID == 0 {
			data.Error = "Veuillez sélectionner un membre."
			renderRightsAdd(c, data)
			return
		}

		if isSiteAdmin(h.db, uint(userID)) {
			data.Error = "Les droits du superadmin global ne sont pas modifiables."
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

		if c.PostForm("right_group_admin") != "" {
			addRight(model.RightGroupAdmin)
		}
		if c.PostForm("right_membership") != "" {
			addRight(model.RightMembership)
		}
		if c.PostForm("right_messages") != "" {
			addRight(model.RightMessages)
		}
		if c.PostForm("right_database_admin") != "" {
			addRight(model.RightDatabaseAdmin)
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

		import_json, _ := json.Marshal(rights)
		ug.Rights = string(import_json)
		h.db.Save(&ug)
		c.Redirect(http.StatusFound, "/amapadmin/rights")
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
	Member         model.UserGroup
	Catalogs       []model.Catalog
	HasGroupAdmin    bool
	HasMembership    bool
	HasMessages      bool
	HasDatabaseAdmin bool
	HasAllCatalogs   bool
	CatalogRights  map[string]bool
	Error          string
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

	// Les droits du superadmin global ne sont pas modifiables : il a tous les
	// droits par construction (cf. handler.loadGroupAccess).
	if isSiteAdmin(h.db, uint(userID)) {
		c.String(http.StatusForbidden, "les droits du superadmin global ne sont pas modifiables")
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

	fillRightsState := func(rights []model.UserRight) {
		for _, r := range rights {
			switch r.Right {
			case model.RightGroupAdmin:
				data.HasGroupAdmin = true
			case model.RightMembership:
				data.HasMembership = true
			case model.RightMessages:
				data.HasMessages = true
			case model.RightDatabaseAdmin:
				data.HasDatabaseAdmin = true
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
		if c.PostForm("right_group_admin") != "" {
			rights = append(rights, model.UserRight{Right: model.RightGroupAdmin})
		}
		if c.PostForm("right_membership") != "" {
			rights = append(rights, model.UserRight{Right: model.RightMembership})
		}
		if c.PostForm("right_messages") != "" {
			rights = append(rights, model.UserRight{Right: model.RightMessages})
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

		encoded, _ := json.Marshal(rights)
		ug.Rights = string(encoded)
		h.db.Save(&ug)
		c.Redirect(http.StatusFound, "/amapadmin/rights")
		return
	}

	fillRightsState(ug.GetRights())

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_rights_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- AmapAdmin shared page data ----

type AmapAdminPageData struct {
	PageData
	AmapAdminTab string
}

func (h *PagesHandler) buildAmapAdminData(c *gin.Context, tab string) (AmapAdminPageData, bool) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.Redirect(http.StatusFound, "/home")
		return AmapAdminPageData{}, false
	}
	pd.Category = "amapadmin"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}
	return AmapAdminPageData{PageData: pd, AmapAdminTab: tab}, true
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
