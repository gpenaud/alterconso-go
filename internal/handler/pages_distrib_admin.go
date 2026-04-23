package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
)

// ---- /distribution/validate/:multiDistribId ----

type ValidateDistribData struct {
	PageData
	MultiDistrib model.MultiDistrib
	Date         string
	Place        string
	Confirmed    bool
	Users        []ValidateUserRow
}

type ValidateUserRow struct {
	UserID    uint
	UserName  string
	Validated bool
	Total     float64
}

func (h *PagesHandler) DistributionValidatePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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

	// Handle validate action
	if c.Query("action") == "validate" {
		h.db.Model(&md).Update("validated", true)
		c.Redirect(http.StatusFound, "/distribution/validate/"+c.Param("id"))
		return
	}

	// Gather all users who have orders for this multiDistrib
	var orders []model.UserOrder
	h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ?", md.ID).
		Preload("User").
		Find(&orders)

	// Group by user
	type userTotal struct {
		name  string
		total float64
	}
	userMap := make(map[uint]*userTotal)
	userList := []uint{}
	for _, o := range orders {
		if _, ok := userMap[o.UserID]; !ok {
			userMap[o.UserID] = &userTotal{
				name: o.User.FirstName + " " + o.User.LastName,
			}
			userList = append(userList, o.UserID)
		}
		userMap[o.UserID].total += o.TotalPrice()
	}

	data := ValidateDistribData{
		PageData:     pd,
		MultiDistrib: md,
		Date:         md.DistribStartDate.Format("02/01/2006"),
		Place:        md.Place.Name,
		Confirmed:    md.Validated,
	}
	data.Title = "Valider la distribution du " + data.Date

	for _, uid := range userList {
		ut := userMap[uid]
		data.Users = append(data.Users, ValidateUserRow{
			UserID:    uid,
			UserName:  ut.name,
			Validated: md.Validated,
			Total:     ut.total,
		})
	}

	t, err := loadTemplates("base.html", "design.html", "distribution_validate.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /distribution/volunteersCalendar ----

var volCalFrDays = [7]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
var volCalFrMonths = [12]string{"Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}

func frDateLabel(t time.Time) string {
	return volCalFrDays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + " " + volCalFrMonths[t.Month()-1]
}

func frDateLabelFull(t time.Time) string {
	return volCalFrDays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + " " + volCalFrMonths[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

type VolunteersCalendarData struct {
	PageData
	From        string
	To          string
	FromLabel   string
	ToLabel     string
	Done        int
	ToBeDone    int
	PeriodStart string
	PeriodEnd   string
	Columns     []VolCalColumn
	Roles       []VolCalRoleRow
}

type VolCalColumn struct {
	ID         uint
	DateLabel  string
	HourLabel  string
	Registered int
	Required   int
	NeedsHelp  bool
}

type VolCalCell struct {
	MultiDistribID uint
	RoleName       string
	VolunteerID    uint
	VolunteerName  string
	IsCurrentUser  bool
	CanJoin        bool
}

type VolCalRoleRow struct {
	Name  string
	Cells []VolCalCell
}

func (h *PagesHandler) VolunteersCalendarPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	// Default: current week (Sunday → Sunday+7)
	now := time.Now()
	daysSinceSunday := int(now.Weekday())
	defaultFrom := now.AddDate(0, 0, -daysSinceSunday)
	defaultTo := defaultFrom.AddDate(0, 0, 7)

	fromStr := c.DefaultQuery("from", defaultFrom.Format("2006-01-02"))
	toStr := c.DefaultQuery("to", defaultTo.Format("2006-01-02"))

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		from = defaultFrom
		fromStr = from.Format("2006-01-02")
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		to = defaultTo
		toStr = to.Format("2006-01-02")
	}

	// Load MultiDistribs in range
	var mds []model.MultiDistrib
	h.db.Where("group_id = ? AND distrib_start_date >= ? AND distrib_start_date <= ?",
		pd.Group.ID, from, to).
		Preload("Place").
		Order("distrib_start_date ASC").
		Find(&mds)

	// Collect MultiDistrib IDs and catalog IDs present in those distribs
	mdIDs := make([]uint, len(mds))
	for i, md := range mds {
		mdIDs[i] = md.ID
	}

	// Load catalog IDs that have a distribution in this period
	activeCatalogIDs := map[uint]bool{}
	if len(mdIDs) > 0 {
		var distribs []model.Distribution
		h.db.Where("multi_distrib_id IN ?", mdIDs).Find(&distribs)
		for _, d := range distribs {
			activeCatalogIDs[d.CatalogID] = true
		}
	}

	// Load VolunteerRoles for the group, restricted to active catalogs
	var roles []model.VolunteerRole
	if len(activeCatalogIDs) > 0 {
		catIDs := make([]uint, 0, len(activeCatalogIDs))
		for id := range activeCatalogIDs {
			catIDs = append(catIDs, id)
		}
		h.db.Where("group_id = ? AND catalog_id IN ?", pd.Group.ID, catIDs).Find(&roles)
	}
	var vols []model.Volunteer
	if len(mdIDs) > 0 {
		h.db.Where("multi_distrib_id IN ?", mdIDs).Preload("User").Find(&vols)
	}

	// Build columns (one per MultiDistrib)
	columns := make([]VolCalColumn, len(mds))
	for i, md := range mds {
		registered := 0
		for _, v := range vols {
			if v.MultiDistribID == md.ID {
				registered++
			}
		}
		required := len(roles)
		if required == 0 {
			required = 1
		}
		columns[i] = VolCalColumn{
			ID:         md.ID,
			DateLabel:  frDateLabel(md.DistribStartDate),
			HourLabel:  md.DistribStartDate.Format("15:04"),
			Registered: registered,
			Required:   required,
			NeedsHelp:  registered < required,
		}
	}

	// Build role rows (one per VolunteerRole)
	roleRows := make([]VolCalRoleRow, len(roles))
	done := 0
	for ri, role := range roles {
		cells := make([]VolCalCell, len(mds))
		for ci, md := range mds {
			cell := VolCalCell{
				MultiDistribID: md.ID,
				RoleName:       role.Name,
				CanJoin:        true,
			}
			for _, v := range vols {
				if v.MultiDistribID == md.ID && v.Role != nil && *v.Role == role.Name {
					cell.VolunteerID = v.ID
					cell.VolunteerName = v.User.FirstName + " " + v.User.LastName
					cell.IsCurrentUser = v.UserID == pd.User.ID
					cell.CanJoin = false
					if v.UserID == pd.User.ID {
						done++
					}
					break
				}
			}
			cells[ci] = cell
		}
		roleRows[ri] = VolCalRoleRow{Name: role.Name, Cells: cells}
	}

	data := VolunteersCalendarData{
		PageData:    pd,
		From:        fromStr,
		To:          toStr,
		FromLabel:   frDateLabelFull(from),
		ToLabel:     frDateLabelFull(to),
		Done:        done,
		ToBeDone:    0,
		PeriodStart: frDateLabel(from),
		PeriodEnd:   frDateLabel(to),
		Columns:     columns,
		Roles:       roleRows,
	}
	data.Title = "Calendrier des permanences"
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Distributions", Link: "/distribution"}}

	t, err2 := loadTemplates("base.html", "design.html", "distribution_volunteers_calendar.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- POST /distribution/volunteersCalendar/join ----

func (h *PagesHandler) VolunteersCalendarJoin(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	mdIDStr := c.PostForm("multiDistribId")
	roleName := c.PostForm("role")
	fromStr := c.PostForm("from")
	toStr := c.PostForm("to")

	mdID, err := strconv.ParseUint(mdIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/distribution/volunteersCalendar?from="+fromStr+"&to="+toStr)
		return
	}

	vol := model.Volunteer{
		UserID:         pd.User.ID,
		MultiDistribID: uint(mdID),
		Role:           &roleName,
	}
	h.db.Create(&vol)

	c.Redirect(http.StatusFound, "/distribution/volunteersCalendar?from="+fromStr+"&to="+toStr)
}

// ---- POST /distribution/volunteersCalendar/leave ----

func (h *PagesHandler) VolunteersCalendarLeave(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	volIDStr := c.PostForm("volunteerId")
	fromStr := c.PostForm("from")
	toStr := c.PostForm("to")

	volID, err := strconv.ParseUint(volIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/distribution/volunteersCalendar?from="+fromStr+"&to="+toStr)
		return
	}

	// Only delete if it belongs to the current user
	h.db.Where("id = ? AND user_id = ?", uint(volID), pd.User.ID).Delete(&model.Volunteer{})

	c.Redirect(http.StatusFound, "/distribution/volunteersCalendar?from="+fromStr+"&to="+toStr)
}

// ---- /distribution/list/:distribId  (printable) ----

type DistribListData struct {
	CatalogName  string
	VendorName   string
	GroupName    string
	Date         string
	StartHour    string
	EndHour      string
	Place        string
	ContactName  string
	ContactEmail string
	ContactPhone string
	TxtDistrib   string
	Volunteers   []string
	UserOrders   []DistribListUserBlock
	GrandTotal   float64
}

type DistribListUserBlock struct {
	UserName   string
	UserPhone  string
	Lines      []PrintOrderLine
	UserTotal  float64
}

type PrintOrderLine struct {
	SmartQty     string
	ProductName  string
	ProductPrice float64
	SubTotal     float64
	Fees         float64
	Total        float64
}

func (h *PagesHandler) DistributionListPage(c *gin.Context) {
	pd := h.buildPageData(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var distrib model.Distribution
	if err := h.db.Preload("MultiDistrib").Preload("MultiDistrib.Place").
		Preload("Catalog").Preload("Catalog.Vendor").Preload("Catalog.Group").
		Preload("Catalog.Contact").
		First(&distrib, id).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	// Check access
	if pd.Group != nil && distrib.Catalog.GroupID != pd.Group.ID {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	var vols []model.Volunteer
	h.db.Where("multi_distrib_id = ?", distrib.MultiDistribID).Preload("User").Find(&vols)
	volNames := make([]string, 0, len(vols))
	for _, v := range vols {
		name := v.User.FirstName + " " + v.User.LastName
		if v.Role != nil {
			name = *v.Role + " : " + name
		}
		volNames = append(volNames, name)
	}

	var orders []model.UserOrder
	h.db.Where("distribution_id = ?", id).
		Preload("User").
		Preload("Product").
		Order("user_id").
		Find(&orders)

	userMap := make(map[uint]*DistribListUserBlock)
	userOrder := []uint{}
	var grandTotal float64

	for _, o := range orders {
		if _, ok := userMap[o.UserID]; !ok {
			phone := ""
			if o.User.Phone != nil {
				phone = *o.User.Phone
			}
			userMap[o.UserID] = &DistribListUserBlock{
				UserName:  o.User.FirstName + " " + o.User.LastName,
				UserPhone: phone,
			}
			userOrder = append(userOrder, o.UserID)
		}
		fees := o.TotalPrice() - o.Quantity*o.ProductPrice
		line := PrintOrderLine{
			SmartQty:     formatQty(o.Quantity, o.Product.UnitType),
			ProductName:  o.Product.Name,
			ProductPrice: o.ProductPrice,
			SubTotal:     o.Quantity * o.ProductPrice,
			Fees:         fees,
			Total:        o.TotalPrice(),
		}
		userMap[o.UserID].Lines = append(userMap[o.UserID].Lines, line)
		userMap[o.UserID].UserTotal += o.TotalPrice()
		grandTotal += o.TotalPrice()
	}

	userBlocks := make([]DistribListUserBlock, 0, len(userOrder))
	for _, uid := range userOrder {
		userBlocks = append(userBlocks, *userMap[uid])
	}

	contactName, contactEmail, contactPhone := "", "", ""
	if distrib.Catalog.Contact != nil {
		c2 := distrib.Catalog.Contact
		contactName = c2.FirstName + " " + c2.LastName
		contactEmail = c2.Email
		if c2.Phone != nil {
			contactPhone = *c2.Phone
		}
	}

	txtDistrib := ""
	if distrib.Catalog.Group.TxtDistrib != nil {
		txtDistrib = *distrib.Catalog.Group.TxtDistrib
	}

	listData := DistribListData{
		CatalogName:  distrib.Catalog.Name,
		VendorName:   distrib.Catalog.Vendor.Name,
		GroupName:    distrib.Catalog.Group.Name,
		Date:         distrib.MultiDistrib.DistribStartDate.Format("02/01/2006"),
		StartHour:    distrib.MultiDistrib.DistribStartDate.Format("15:04"),
		EndHour:      distrib.MultiDistrib.DistribEndDate.Format("15:04"),
		Place:        distrib.MultiDistrib.Place.Name,
		ContactName:  contactName,
		ContactEmail: contactEmail,
		ContactPhone: contactPhone,
		TxtDistrib:   txtDistrib,
		Volunteers:   volNames,
		UserOrders:   userBlocks,
		GrandTotal:   grandTotal,
	}

	t, err := loadTemplates("distribution_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "distribution_list", listData); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /distribution/inviteFarmers/:multiDistribId ----

func (h *PagesHandler) DistributionInviteFarmersPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	mdID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	var md model.MultiDistrib
	if err := h.db.Preload("Place").Preload("Distributions.Catalog.Vendor").
		First(&md, mdID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	type CatalogRow struct {
		ID         uint
		Name       string
		VendorName string
		Active     bool
	}

	// All catalogs of the group
	var allCatalogs []model.Catalog
	h.db.Where("group_id = ?", pd.Group.ID).Preload("Vendor").Find(&allCatalogs)

	// Active catalog IDs for this multidistrib
	activeIDs := map[uint]bool{}
	for _, d := range md.Distributions {
		activeIDs[d.CatalogID] = true
	}

	rows := make([]CatalogRow, 0, len(allCatalogs))
	for _, cat := range allCatalogs {
		rows = append(rows, CatalogRow{
			ID:         cat.ID,
			Name:       cat.Name,
			VendorName: cat.Vendor.Name,
			Active:     activeIDs[cat.ID],
		})
	}

	type pageData struct {
		PageData
		MultiDistrib model.MultiDistrib
		Date         string
		Catalogs     []CatalogRow
	}
	data := pageData{
		PageData:     pd,
		MultiDistrib: md,
		Date:         md.DistribStartDate.Format("02/01/2006"),
		Catalogs:     rows,
	}
	data.Title = "Producteurs participants"
	data.Category = "distribution"

	t, err2 := loadTemplates("base.html", "design.html", "distribution_invite_farmers.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- /distribution/notAttend/:distribId ----

func (h *PagesHandler) DistributionNotAttendPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	distribID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	var distrib model.Distribution
	if err := h.db.Preload("Catalog").First(&distrib, distribID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}
	if distrib.Catalog.GroupID != pd.Group.ID {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	// Delete the distribution (remove catalog from multidistrib)
	h.db.Delete(&distrib)
	from := c.DefaultQuery("from", "/distribution")
	if from == "distribSection" {
		from = "/distribution"
	}
	c.Redirect(http.StatusFound, from)
}

// ---- /distribution/shift/:distribId ----

func (h *PagesHandler) DistributionShiftPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	distribID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	var distrib model.Distribution
	if err := h.db.Preload("MultiDistrib").Preload("Catalog").First(&distrib, distribID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	type pageData struct {
		PageData
		Distribution model.Distribution
		CurrentDate  string
		NewDate      string
	}
	data := pageData{
		PageData:     pd,
		Distribution: distrib,
		CurrentDate:  distrib.MultiDistrib.DistribStartDate.Format("2006-01-02"),
	}
	data.Title = "Reporter la distribution"
	data.Category = "distribution"

	if c.Request.Method == "POST" {
		newDateStr := c.PostForm("newDate")
		newDate, err := time.Parse("2006-01-02", newDateStr)
		if err != nil {
			c.String(http.StatusBadRequest, "date invalide")
			return
		}
		distrib.Date = &newDate
		h.db.Save(&distrib)
		c.Redirect(http.StatusFound, "/distribution")
		return
	}

	t, err2 := loadTemplates("base.html", "design.html", "distribution_shift.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- /edit/:distribId ----

func (h *PagesHandler) DistributionEditDatesPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	distribID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	var distrib model.Distribution
	if err := h.db.Preload("MultiDistrib").Preload("Catalog").First(&distrib, distribID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	type pageData struct {
		PageData
		Distribution   model.Distribution
		OrderStartDate string
		OrderEndDate   string
	}

	orderStart := ""
	if distrib.OrderStartDate != nil {
		orderStart = distrib.OrderStartDate.Format("2006-01-02T15:04")
	} else if distrib.MultiDistrib.OrderStartDate != nil {
		orderStart = distrib.MultiDistrib.OrderStartDate.Format("2006-01-02T15:04")
	}
	orderEnd := ""
	if distrib.OrderEndDate != nil {
		orderEnd = distrib.OrderEndDate.Format("2006-01-02T15:04")
	} else if distrib.MultiDistrib.OrderEndDate != nil {
		orderEnd = distrib.MultiDistrib.OrderEndDate.Format("2006-01-02T15:04")
	}

	data := pageData{
		PageData:       pd,
		Distribution:   distrib,
		OrderStartDate: orderStart,
		OrderEndDate:   orderEnd,
	}
	data.Title = "Personnaliser les dates"
	data.Category = "distribution"

	if c.Request.Method == "POST" {
		startStr := c.PostForm("orderStartDate")
		endStr := c.PostForm("orderEndDate")
		if t, err := time.ParseInLocation("2006-01-02T15:04", startStr, time.Local); err == nil {
			distrib.OrderStartDate = &t
		}
		if t, err := time.ParseInLocation("2006-01-02T15:04", endStr, time.Local); err == nil {
			distrib.OrderEndDate = &t
		}
		h.db.Save(&distrib)
		c.Redirect(http.StatusFound, "/distribution")
		return
	}

	t2, err2 := loadTemplates("base.html", "design.html", "distribution_edit_dates.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t2.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- GET/POST /distribution/editMd/:id ----

type EditMdData struct {
	PageData
	MultiDistrib    model.MultiDistrib
	Places          []model.Place
	DateLabel       string
	DefaultStart    string
	DefaultEnd      string
	DefaultOrdOpen  string
	DefaultOrdClose string
}

func (h *PagesHandler) DistributionEditMdPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var md model.MultiDistrib
	if err := h.db.Preload("Place").First(&md, id).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}
	if md.GroupID != pd.Group.ID {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	var places []model.Place
	h.db.Where("group_id = ?", pd.Group.ID).Find(&places)

	if c.Request.Method == "POST" {
		startHour := c.PostForm("startHour")
		endHour := c.PostForm("endHour")
		ordOpen := c.PostForm("orderStartDate")
		ordClose := c.PostForm("orderEndDate")
		placeIDStr := c.PostForm("placeId")
		syncAll := c.PostForm("syncAll") == "on"

		dateStr := md.DistribStartDate.Format("2006-01-02")
		if t, err := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+startHour, time.Local); err == nil {
			md.DistribStartDate = t
		}
		if t, err := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+endHour, time.Local); err == nil {
			md.DistribEndDate = t
		}
		if t, err := time.ParseInLocation("2006-01-02T15:04", ordOpen, time.Local); err == nil {
			md.OrderStartDate = &t
		}
		if t, err := time.ParseInLocation("2006-01-02T15:04", ordClose, time.Local); err == nil {
			md.OrderEndDate = &t
		}
		if placeID, err := strconv.ParseUint(placeIDStr, 10, 64); err == nil {
			md.PlaceID = uint(placeID)
		}
		result := h.db.Model(&model.MultiDistrib{}).Where("id = ?", md.ID).Updates(map[string]interface{}{
			"distrib_start_date": md.DistribStartDate,
			"distrib_end_date":   md.DistribEndDate,
			"order_start_date":   md.OrderStartDate,
			"order_end_date":     md.OrderEndDate,
			"place_id":           md.PlaceID,
		})
		if result.Error != nil {
			c.String(http.StatusInternalServerError, "erreur sauvegarde: %v", result.Error)
			return
		}

		if syncAll {
			// Mettre à jour toutes les distributions liées
			h.db.Model(&model.Distribution{}).
				Where("multi_distrib_id = ?", md.ID).
				Updates(map[string]interface{}{
					"order_start_date": md.OrderStartDate,
					"order_end_date":   md.OrderEndDate,
				})
		}

		c.Redirect(http.StatusFound, "/distribution")
		return
	}

	frDays := [7]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frMonths := [12]string{"Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	t := md.DistribStartDate
	dateLabel := frDays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + " " + frMonths[t.Month()-1] + " " + strconv.Itoa(t.Year())

	ordOpen := ""
	if md.OrderStartDate != nil {
		ordOpen = md.OrderStartDate.Format("2006-01-02T15:04")
	}
	ordClose := ""
	if md.OrderEndDate != nil {
		ordClose = md.OrderEndDate.Format("2006-01-02T15:04")
	}

	data := EditMdData{
		PageData:        pd,
		MultiDistrib:    md,
		Places:          places,
		DateLabel:       dateLabel,
		DefaultStart:    md.DistribStartDate.Format("15:04"),
		DefaultEnd:      md.DistribEndDate.Format("15:04"),
		DefaultOrdOpen:  ordOpen,
		DefaultOrdClose: ordClose,
	}
	data.Title = "Modifier une distribution"
	data.Category = "distribution"

	tmpl, err := loadTemplates("base.html", "design.html", "distribution_edit_md.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := tmpl.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET /distribution/deleteMd/:id ----

func (h *PagesHandler) DistributionDeleteMdPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	var md model.MultiDistrib
	if err := h.db.First(&md, id).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}
	if md.GroupID != pd.Group.ID {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	// Supprimer les distributions liées puis le MultiDistrib
	h.db.Where("multi_distrib_id = ?", md.ID).Delete(&model.Distribution{})
	h.db.Delete(&md)
	c.Redirect(http.StatusFound, "/distribution")
}

// ---- GET/POST /distribution/insertMd ----

type InsertMdData struct {
	PageData
	Places         []model.Place
	DefaultDate    string
	DefaultStart   string
	DefaultEnd     string
	DefaultOrdOpen string
	DefaultOrdClose string
}

func (h *PagesHandler) DistributionInsertMdPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	var places []model.Place
	h.db.Where("group_id = ?", pd.Group.ID).Find(&places)

	if c.Request.Method == "POST" {
		dateStr := c.PostForm("date")
		startHour := c.PostForm("startHour")
		endHour := c.PostForm("endHour")
		ordOpen := c.PostForm("orderStartDate")
		ordClose := c.PostForm("orderEndDate")
		placeIDStr := c.PostForm("placeId")

		distribStart, err1 := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+startHour, time.Local)
		distribEnd, err2 := time.ParseInLocation("2006-01-02T15:04", dateStr+"T"+endHour, time.Local)
		placeID, err3 := strconv.ParseUint(placeIDStr, 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			c.String(http.StatusBadRequest, "paramètres invalides")
			return
		}

		md := model.MultiDistrib{
			GroupID:          pd.Group.ID,
			PlaceID:          uint(placeID),
			DistribStartDate: distribStart,
			DistribEndDate:   distribEnd,
		}
		if t, err := time.ParseInLocation("2006-01-02T15:04", ordOpen, time.Local); err == nil {
			md.OrderStartDate = &t
		}
		if t, err := time.ParseInLocation("2006-01-02T15:04", ordClose, time.Local); err == nil {
			md.OrderEndDate = &t
		}
		h.db.Create(&md)
		c.Redirect(http.StatusFound, "/distribution")
		return
	}

	now := time.Now()
	data := InsertMdData{
		PageData:        pd,
		Places:          places,
		DefaultDate:     now.AddDate(0, 0, 30).Format("2006-01-02"),
		DefaultStart:    "19:00",
		DefaultEnd:      "20:00",
		DefaultOrdOpen:  now.AddDate(0, 0, 10).Format("2006-01-02") + "T08:00",
		DefaultOrdClose: now.AddDate(0, 0, 20).Format("2006-01-02") + "T23:59",
	}
	data.Title = "Créer une distribution générale"
	data.Category = "distribution"

	t, err := loadTemplates("base.html", "design.html", "distribution_insert_md.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /distribution/insertMdCycle ----

type InsertMdCycleData struct {
	PageData
	Places       []model.Place
	DefaultStart string
	DefaultEnd   string
}

func (h *PagesHandler) DistributionInsertMdCyclePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	var places []model.Place
	h.db.Where("group_id = ?", pd.Group.ID).Find(&places)

	if c.Request.Method == "POST" {
		cycleType := c.PostForm("cycleType")
		startDateStr := c.PostForm("startDate")
		endDateStr := c.PostForm("endDate")
		startHour := c.PostForm("startHour")
		endHour := c.PostForm("endHour")
		daysBeforeOpenStr := c.PostForm("daysBeforeOpen")
		openingHour := c.PostForm("openingHour")
		daysBeforeCloseStr := c.PostForm("daysBeforeClose")
		closingHour := c.PostForm("closingHour")
		placeIDStr := c.PostForm("placeId")

		startDate, err1 := time.ParseInLocation("2006-01-02", startDateStr, time.Local)
		endDate, err2 := time.ParseInLocation("2006-01-02", endDateStr, time.Local)
		placeID, err3 := strconv.ParseUint(placeIDStr, 10, 64)
		daysBeforeOpen, err4 := strconv.Atoi(daysBeforeOpenStr)
		daysBeforeClose, err5 := strconv.Atoi(daysBeforeCloseStr)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
			c.String(http.StatusBadRequest, "paramètres invalides")
			return
		}

		// Calcul de l'intervalle selon le type de cycle
		var interval int
		switch cycleType {
		case "Weekly":
			interval = 7
		case "BiWeekly":
			interval = 14
		case "TriWeekly":
			interval = 21
		case "Monthly":
			interval = 30
		default:
			interval = 7
		}

		// Génération des MultiDistribs
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, interval) {
			distribStart, _ := time.ParseInLocation("2006-01-02T15:04", d.Format("2006-01-02")+"T"+startHour, time.Local)
			distribEnd, _ := time.ParseInLocation("2006-01-02T15:04", d.Format("2006-01-02")+"T"+endHour, time.Local)
			ordOpenDate := d.AddDate(0, 0, -daysBeforeOpen)
			ordCloseDate := d.AddDate(0, 0, -daysBeforeClose)
			ordOpen, _ := time.ParseInLocation("2006-01-02T15:04", ordOpenDate.Format("2006-01-02")+"T"+openingHour, time.Local)
			ordClose, _ := time.ParseInLocation("2006-01-02T15:04", ordCloseDate.Format("2006-01-02")+"T"+closingHour, time.Local)

			md := model.MultiDistrib{
				GroupID:          pd.Group.ID,
				PlaceID:          uint(placeID),
				DistribStartDate: distribStart,
				DistribEndDate:   distribEnd,
				OrderStartDate:   &ordOpen,
				OrderEndDate:     &ordClose,
			}
			h.db.Create(&md)
		}

		c.Redirect(http.StatusFound, "/distribution")
		return
	}

	now := time.Now()
	data := InsertMdCycleData{
		PageData:     pd,
		Places:       places,
		DefaultStart: now.Format("2006-01-02"),
		DefaultEnd:   now.AddDate(0, 1, 0).Format("2006-01-02"),
	}
	data.Title = "Programmer un cycle de distribution"
	data.Category = "distribution"

	t, err := loadTemplates("base.html", "design.html", "distribution_insert_md_cycle.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /distribution/roles/:id ----

type DistribRolesData struct {
	PageData
	MultiDistrib model.MultiDistrib
	DateLabel    string
	Roles        []DistribRoleItem
}

type DistribRoleItem struct {
	ID       uint
	Name     string
	Catalog  string
	Selected bool
}

func (h *PagesHandler) DistribRolesPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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

	// Collect catalog IDs participating in this distribution
	catalogIDs := make([]uint, 0, len(md.Distributions))
	for _, d := range md.Distributions {
		catalogIDs = append(catalogIDs, d.CatalogID)
	}

	// Load only volunteer roles for catalogs in this distribution
	var roles []model.VolunteerRole
	if len(catalogIDs) > 0 {
		h.db.Where("group_id = ? AND catalog_id IN ?", pd.Group.ID, catalogIDs).Preload("Catalog").Find(&roles)
	}

	// Load already selected roles
	var selected []model.MultiDistribRole
	h.db.Where("multi_distrib_id = ?", md.ID).Find(&selected)
	selectedSet := map[uint]bool{}
	for _, s := range selected {
		selectedSet[s.VolunteerRoleID] = true
	}

	if c.Request.Method == http.MethodPost {
		// Delete all existing selections
		h.db.Where("multi_distrib_id = ?", md.ID).Delete(&model.MultiDistribRole{})
		// Re-insert checked ones
		for _, r := range roles {
			if c.PostForm("role_"+strconv.Itoa(int(r.ID))) == "1" {
				h.db.Create(&model.MultiDistribRole{
					MultiDistribID:  md.ID,
					VolunteerRoleID: r.ID,
				})
			}
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

	data := DistribRolesData{
		PageData:     pd,
		MultiDistrib: md,
		DateLabel:    dateLabel,
	}
	data.Title = "Rôles de bénévoles"

	for _, r := range roles {
		item := DistribRoleItem{
			ID:       r.ID,
			Name:     r.Name,
			Selected: selectedSet[r.ID],
		}
		if r.Catalog != nil {
			item.Catalog = r.Catalog.Name
		}
		data.Roles = append(data.Roles, item)
	}

	t, err := loadTemplates("base.html", "design.html", "distribution_roles.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
