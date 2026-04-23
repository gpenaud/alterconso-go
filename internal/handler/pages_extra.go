package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
	UserID uint
	Name   string
	Rights []string
}

func (h *PagesHandler) AmapAdminRightsPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "rights")
	if !ok {
		return
	}

	var ugs []model.UserGroup
	h.db.Where("group_id = ?", base.Group.ID).Preload("User").Find(&ugs)

	data := AmapAdminRightsData{AmapAdminPageData: base}
	data.Title = "Droits d'administration"

	for _, ug := range ugs {
		rights := ug.GetRights()
		if len(rights) == 0 {
			continue
		}
		rv := RightUserView{
			UserID: ug.UserID,
			Name:   ug.User.FirstName + " " + ug.User.LastName,
		}
		for _, r := range rights {
			rv.Rights = append(rv.Rights, string(r.Right))
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

// ---- AmapAdmin shared page data ----

type AmapAdminPageData struct {
	PageData
	AmapAdminTab string
}

func (h *PagesHandler) buildAmapAdminData(c *gin.Context, tab string) (AmapAdminPageData, bool) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return AmapAdminPageData{}, false
	}
	pd.Category = "amapadmin"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}
	return AmapAdminPageData{PageData: pd, AmapAdminTab: tab}, true
}

// ---- GET /amapadmin/vatRates ----

type VatRatesData struct {
	AmapAdminPageData
	VatNames [4]string
	VatRates [4]float64
}

func (h *PagesHandler) AmapAdminVatRatesPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "vatRates")
	if !ok {
		return
	}
	data := VatRatesData{AmapAdminPageData: base}
	data.Title = "Taux de TVA"
	g := base.Group
	data.VatNames = [4]string{g.VatName1, g.VatName2, g.VatName3, g.VatName4}
	data.VatRates = [4]float64{g.VatRate1, g.VatRate2, g.VatRate3, g.VatRate4}

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
	parseRate := func(s string) float64 {
		var f float64
		fmt.Sscanf(s, "%f", &f)
		return f
	}
	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(map[string]interface{}{
		"vat_name1": c.PostForm("name1"), "vat_rate1": parseRate(c.PostForm("rate1")),
		"vat_name2": c.PostForm("name2"), "vat_rate2": parseRate(c.PostForm("rate2")),
		"vat_name3": c.PostForm("name3"), "vat_rate3": parseRate(c.PostForm("rate3")),
		"vat_name4": c.PostForm("name4"), "vat_rate4": parseRate(c.PostForm("rate4")),
	})
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
	hasMembership := c.PostForm("hasMembership") == "1"
	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(map[string]interface{}{
		"has_membership": hasMembership,
	})
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

func (h *PagesHandler) AmapAdminDocumentsPage(c *gin.Context) {
	base, ok := h.buildAmapAdminData(c, "documents")
	if !ok {
		return
	}
	base.Title = "Documents"

	t, err := loadTemplates("base.html", "design.html", "amapadmin_layout.html", "amapadmin_documents.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", base); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
