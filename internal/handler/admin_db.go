package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
)

// ==================== Registry ====================

// adminDBModel describes a GORM model exposed in the DB admin.
type adminDBModel struct {
	Slug             string
	Label            string
	New              func() any // pointer to a zero value, e.g. &model.User{}
	NewSlice         func() any // pointer to empty slice, e.g. &[]model.User{}
	ListFields       []string   // Go struct field names shown in the listing
	EditFields       []string   // Go struct field names editable via form
	SearchableCols   []string   // DB column names (snake_case) for LIKE search
	OrderBy          string     // DB order clause, default "id DESC"
}

var adminDBRegistry = []adminDBModel{
	{
		Slug: "users", Label: "Utilisateurs",
		New: func() any { return &model.User{} }, NewSlice: func() any { return &[]model.User{} },
		ListFields:     []string{"ID", "Email", "FirstName", "LastName", "Phone", "Lang"},
		EditFields:     []string{"FirstName", "LastName", "Email", "Phone", "FirstName2", "LastName2", "Email2", "Phone2", "Address1", "Address2", "ZipCode", "City", "Nationality", "CountryOfResidence", "Lang"},
		SearchableCols: []string{"email", "first_name", "last_name"},
	},
	{
		Slug: "groups", Label: "Groupes",
		New: func() any { return &model.Group{} }, NewSlice: func() any { return &[]model.Group{} },
		ListFields:     []string{"ID", "Name", "GroupType", "RegOption"},
		EditFields:     []string{"Name", "GroupType", "RegOption", "TxtIntro", "TxtHome", "TxtDistrib", "ExtURL", "ContactID", "LegalRepresentativeID", "HasMembership", "MembershipFee", "Currency", "CurrencyCode", "Flags"},
		SearchableCols: []string{"name"},
	},
	{
		Slug: "vendors", Label: "Producteurs",
		New: func() any { return &model.Vendor{} }, NewSlice: func() any { return &[]model.Vendor{} },
		ListFields:     []string{"ID", "Name", "Email", "Phone", "City"},
		EditFields:     []string{"Name", "Email", "Phone", "Address1", "ZipCode", "City", "Description", "Organic"},
		SearchableCols: []string{"name", "email", "city"},
	},
	{
		Slug: "catalogs", Label: "Catalogues",
		New: func() any { return &model.Catalog{} }, NewSlice: func() any { return &[]model.Catalog{} },
		ListFields:     []string{"ID", "Name", "GroupID", "VendorID", "Type", "StartDate", "EndDate"},
		EditFields:     []string{"Name", "Type", "StartDate", "EndDate", "PercentageFees", "PercentageName", "VendorID", "GroupID", "ContactID", "Flags"},
		SearchableCols: []string{"name"},
	},
	{
		Slug: "products", Label: "Produits",
		New: func() any { return &model.Product{} }, NewSlice: func() any { return &[]model.Product{} },
		ListFields:     []string{"ID", "Name", "Ref", "CatalogID", "Price", "UnitType", "Active"},
		EditFields:     []string{"Name", "Ref", "Description", "Qt", "Price", "VAT", "UnitType", "Organic", "VariablePrice", "MultiWeight", "HasFloatQt", "Active", "Stock", "StockTracked", "CatalogID", "CategoryID"},
		SearchableCols: []string{"name", "ref"},
	},
	{
		Slug: "multi-distribs", Label: "Multi-distributions",
		New: func() any { return &model.MultiDistrib{} }, NewSlice: func() any { return &[]model.MultiDistrib{} },
		ListFields:     []string{"ID", "GroupID", "PlaceID", "DistribStartDate", "Validated"},
		EditFields:     []string{"GroupID", "PlaceID", "DistribStartDate", "DistribEndDate", "OrderStartDate", "OrderEndDate", "Validated"},
		OrderBy:        "distrib_start_date DESC",
	},
	{
		Slug: "distributions", Label: "Distributions (par catalogue)",
		New: func() any { return &model.Distribution{} }, NewSlice: func() any { return &[]model.Distribution{} },
		ListFields:     []string{"ID", "CatalogID", "MultiDistribID", "Date"},
		EditFields:     []string{"CatalogID", "MultiDistribID", "Date", "End", "OrderStartDate", "OrderEndDate"},
	},
	{
		Slug: "places", Label: "Lieux",
		New: func() any { return &model.Place{} }, NewSlice: func() any { return &[]model.Place{} },
		ListFields:     []string{"ID", "Name", "City", "GroupID"},
		EditFields:     []string{"Name", "Address", "ZipCode", "City", "Lat", "Lng", "GroupID"},
		SearchableCols: []string{"name", "city"},
	},
	{
		Slug: "volunteer-roles", Label: "Rôles bénévoles",
		New: func() any { return &model.VolunteerRole{} }, NewSlice: func() any { return &[]model.VolunteerRole{} },
		ListFields:     []string{"ID", "Name", "GroupID", "CatalogID"},
		EditFields:     []string{"Name", "GroupID", "CatalogID"},
		SearchableCols: []string{"name"},
	},
	{
		Slug: "volunteers", Label: "Bénévoles inscrits",
		New: func() any { return &model.Volunteer{} }, NewSlice: func() any { return &[]model.Volunteer{} },
		ListFields:     []string{"ID", "UserID", "MultiDistribID", "Role"},
		EditFields:     []string{"UserID", "MultiDistribID", "Role"},
	},
	{
		Slug: "user-orders", Label: "Commandes",
		New: func() any { return &model.UserOrder{} }, NewSlice: func() any { return &[]model.UserOrder{} },
		ListFields:     []string{"ID", "UserID", "ProductID", "Quantity", "ProductPrice", "Paid", "DistributionID"},
		EditFields:     []string{"Quantity", "ProductPrice", "FeesRate", "Paid", "DistributionID", "BasketID", "SubscriptionID"},
	},
	{
		Slug: "memberships", Label: "Adhésions",
		New: func() any { return &model.Membership{} }, NewSlice: func() any { return &[]model.Membership{} },
		ListFields:     []string{"ID", "UserID", "GroupID", "Year", "Fee"},
		EditFields:     []string{"UserID", "GroupID", "Year", "Fee"},
	},
	{
		Slug: "group-docs", Label: "Documents (fichiers PDF)",
		New: func() any { return &model.GroupDoc{} }, NewSlice: func() any { return &[]model.GroupDoc{} },
		ListFields:     []string{"ID", "GroupID", "Name", "FileID"},
		EditFields:     []string{"Name", "GroupID", "FileID"},
		SearchableCols: []string{"name"},
	},
	{
		Slug: "files", Label: "Fichiers (métadonnées)",
		New: func() any { return &model.File{} }, NewSlice: func() any { return &[]model.File{} },
		ListFields:     []string{"ID", "Name"},
		EditFields:     []string{"Name"}, // Data BLOB jamais exposé
		SearchableCols: []string{"name"},
	},
	{
		Slug: "messages", Label: "Messages",
		New: func() any { return &model.Message{} }, NewSlice: func() any { return &[]model.Message{} },
		ListFields:     []string{"ID", "SenderID", "GroupID", "Subject"},
		EditFields:     []string{"SenderID", "GroupID", "Subject", "Body"},
	},
	{
		Slug: "subscriptions", Label: "Souscriptions",
		New: func() any { return &model.Subscription{} }, NewSlice: func() any { return &[]model.Subscription{} },
		ListFields:     []string{"ID", "UserID", "CatalogID", "StartDate", "EndDate"},
		EditFields:     []string{"UserID", "CatalogID", "StartDate", "EndDate", "Quantities"},
	},
}

func findAdminModel(slug string) *adminDBModel {
	for i := range adminDBRegistry {
		if adminDBRegistry[i].Slug == slug {
			return &adminDBRegistry[i]
		}
	}
	return nil
}

// ==================== Shared view helpers ====================

type adminDBLayoutData struct {
	PageData
	Tables     []adminDBModel
	ActiveSlug string
	PageTitle  string
}

func (h *PagesHandler) adminDBGuard(c *gin.Context) (PageData, bool) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login?__redirect=/admin/db")
		return pd, false
	}
	if !pd.HasDatabaseAdmin {
		c.String(http.StatusForbidden, "accès refusé")
		return pd, false
	}
	return pd, true
}

// ==================== Handlers ====================

// GET /admin/db — index
func (h *PagesHandler) AdminDBIndex(c *gin.Context) {
	pd, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	pd.Title = "Base de données"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Base de données", Link: "/admin/db"}}

	data := adminDBLayoutData{
		PageData:  pd,
		Tables:    adminDBRegistry,
		PageTitle: "Base de données",
	}

	t, err := loadTemplates("base.html", "design.html", "admin_db_layout.html", "admin_db_index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// adminDBListData for the list view.
type adminDBListRow struct {
	ID     uint
	Cells  []template.HTML
}

type adminDBListData struct {
	adminDBLayoutData
	Model       *adminDBModel
	Headers     []string
	Rows        []adminDBListRow
	Total       int64
	Page        int
	PageCount   int
	Search      string
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
}

// GET /admin/db/:slug
func (h *PagesHandler) AdminDBList(c *gin.Context) {
	pd, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	m := findAdminModel(slug)
	if m == nil {
		c.String(http.StatusNotFound, "table introuvable")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	const pageSize = 50
	search := strings.TrimSpace(c.Query("search"))

	// Count + load
	slice := m.NewSlice()
	q := h.db.Model(m.New())
	if search != "" && len(m.SearchableCols) > 0 {
		ors := make([]string, len(m.SearchableCols))
		args := make([]any, len(m.SearchableCols))
		for i, col := range m.SearchableCols {
			ors[i] = col + " LIKE ?"
			args[i] = "%" + search + "%"
		}
		q = q.Where(strings.Join(ors, " OR "), args...)
	}
	var total int64
	q.Count(&total)
	order := m.OrderBy
	if order == "" {
		order = "id DESC"
	}
	q.Order(order).Limit(pageSize).Offset((page - 1) * pageSize).Find(slice)

	headers := append([]string{}, m.ListFields...)
	var rows []adminDBListRow
	sliceVal := reflect.ValueOf(slice).Elem()
	for i := 0; i < sliceVal.Len(); i++ {
		rec := sliceVal.Index(i).Addr().Interface()
		row := adminDBListRow{ID: getPK(rec)}
		for _, f := range m.ListFields {
			row.Cells = append(row.Cells, renderFieldHTML(rec, f, true))
		}
		rows = append(rows, row)
	}

	pageCount := int((total + pageSize - 1) / pageSize)
	if pageCount < 1 {
		pageCount = 1
	}
	pd.Title = m.Label
	pd.Breadcrumb = []BreadcrumbItem{
		{Name: "Base de données", Link: "/admin/db"},
		{Name: m.Label, Link: "/admin/db/" + m.Slug},
	}

	data := adminDBListData{
		adminDBLayoutData: adminDBLayoutData{PageData: pd, Tables: adminDBRegistry, ActiveSlug: m.Slug, PageTitle: m.Label},
		Model:             m,
		Headers:           headers,
		Rows:              rows,
		Total:             total,
		Page:              page,
		PageCount:         pageCount,
		Search:            search,
		HasPrev:           page > 1,
		HasNext:           page < pageCount,
		PrevPage:          page - 1,
		NextPage:          page + 1,
	}

	t, err := loadTemplates("base.html", "design.html", "admin_db_layout.html", "admin_db_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ==================== Edit view ====================

type adminDBFormField struct {
	Name        string
	Label       string
	InputType   string // text, number, checkbox, datetime-local, textarea, select
	StringValue string
	IsChecked   bool
	IsNullable  bool
	Options     []string // for enum-like string types
}

type adminDBEditData struct {
	adminDBLayoutData
	Model    *adminDBModel
	IsNew    bool
	ID       uint
	Fields   []adminDBFormField
	ErrorMsg string
}

// GET /admin/db/:slug/new
func (h *PagesHandler) AdminDBNew(c *gin.Context) {
	pd, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	m := findAdminModel(slug)
	if m == nil {
		c.String(http.StatusNotFound, "table introuvable")
		return
	}
	rec := m.New()
	h.renderAdminDBEdit(c, pd, m, rec, true, 0, "")
}

// GET /admin/db/:slug/edit/:id
func (h *PagesHandler) AdminDBEdit(c *gin.Context) {
	pd, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	m := findAdminModel(slug)
	if m == nil {
		c.String(http.StatusNotFound, "table introuvable")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	rec := m.New()
	if err := h.db.First(rec, id).Error; err != nil {
		c.String(http.StatusNotFound, "enregistrement introuvable")
		return
	}
	h.renderAdminDBEdit(c, pd, m, rec, false, uint(id), "")
}

func (h *PagesHandler) renderAdminDBEdit(c *gin.Context, pd PageData, m *adminDBModel, rec any, isNew bool, id uint, errMsg string) {
	var fields []adminDBFormField
	for _, fname := range m.EditFields {
		f, ok := formFieldFromStruct(rec, fname)
		if !ok {
			continue
		}
		fields = append(fields, f)
	}

	title := m.Label + " #" + strconv.FormatUint(uint64(id), 10)
	if isNew {
		title = "Nouveau — " + m.Label
	}
	pd.Title = title
	pd.Breadcrumb = []BreadcrumbItem{
		{Name: "Base de données", Link: "/admin/db"},
		{Name: m.Label, Link: "/admin/db/" + m.Slug},
		{Name: title, Link: "#"},
	}

	data := adminDBEditData{
		adminDBLayoutData: adminDBLayoutData{PageData: pd, Tables: adminDBRegistry, ActiveSlug: m.Slug, PageTitle: title},
		Model:             m,
		IsNew:             isNew,
		ID:                id,
		Fields:            fields,
		ErrorMsg:          errMsg,
	}

	t, err := loadTemplates("base.html", "design.html", "admin_db_layout.html", "admin_db_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// POST /admin/db/:slug/new
func (h *PagesHandler) AdminDBCreate(c *gin.Context) {
	pd, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	m := findAdminModel(slug)
	if m == nil {
		c.String(http.StatusNotFound, "table introuvable")
		return
	}
	rec := m.New()
	if err := applyFormToStruct(c, rec, m.EditFields); err != nil {
		h.renderAdminDBEdit(c, pd, m, rec, true, 0, err.Error())
		return
	}
	if err := h.db.Create(rec).Error; err != nil {
		h.renderAdminDBEdit(c, pd, m, rec, true, 0, "Erreur création : "+err.Error())
		return
	}
	id := getPK(rec)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/db/%s/edit/%d?saved=1", m.Slug, id))
}

// POST /admin/db/:slug/edit/:id
func (h *PagesHandler) AdminDBSave(c *gin.Context) {
	pd, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	m := findAdminModel(slug)
	if m == nil {
		c.String(http.StatusNotFound, "table introuvable")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	rec := m.New()
	if err := h.db.First(rec, id).Error; err != nil {
		c.String(http.StatusNotFound, "enregistrement introuvable")
		return
	}
	if err := applyFormToStruct(c, rec, m.EditFields); err != nil {
		h.renderAdminDBEdit(c, pd, m, rec, false, uint(id), err.Error())
		return
	}
	if err := h.db.Save(rec).Error; err != nil {
		h.renderAdminDBEdit(c, pd, m, rec, false, uint(id), "Erreur sauvegarde : "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/db/%s/edit/%d?saved=1", m.Slug, id))
}

// POST /admin/db/:slug/delete/:id
func (h *PagesHandler) AdminDBDelete(c *gin.Context) {
	_, ok := h.adminDBGuard(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	m := findAdminModel(slug)
	if m == nil {
		c.String(http.StatusNotFound, "table introuvable")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	rec := m.New()
	h.db.Delete(rec, id)
	c.Redirect(http.StatusFound, "/admin/db/"+m.Slug)
}

// ==================== Reflection helpers ====================

func getPK(rec any) uint {
	v := reflect.ValueOf(rec)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if !v.IsValid() {
		return 0
	}
	f := v.FieldByName("ID")
	if !f.IsValid() {
		return 0
	}
	switch f.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uint(f.Uint())
	}
	return 0
}

// renderFieldHTML returns a display-safe representation of a field.
// When truncate is true, long strings are cut to ~60 chars.
func renderFieldHTML(rec any, fieldName string, truncate bool) template.HTML {
	v := reflect.ValueOf(rec)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	f := v.FieldByName(fieldName)
	if !f.IsValid() {
		return template.HTML("")
	}
	s := formatReflectValue(f)
	if truncate && len(s) > 60 {
		s = s[:60] + "…"
	}
	return template.HTML(template.HTMLEscapeString(s))
}

func formatReflectValue(f reflect.Value) string {
	if !f.IsValid() {
		return ""
	}
	// Dereference pointer
	if f.Kind() == reflect.Ptr {
		if f.IsNil() {
			return ""
		}
		f = f.Elem()
	}
	// Time
	if t, ok := f.Interface().(time.Time); ok {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04")
	}
	switch f.Kind() {
	case reflect.Bool:
		if f.Bool() {
			return "✓"
		}
		return "—"
	case reflect.String:
		return f.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(f.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(f.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(f.Float(), 'f', -1, 64)
	case reflect.Slice:
		// skip byte slices (blobs), show length otherwise
		if f.Type().Elem().Kind() == reflect.Uint8 {
			return fmt.Sprintf("[%d bytes]", f.Len())
		}
		return fmt.Sprintf("[%d items]", f.Len())
	case reflect.Struct:
		return "{…}"
	}
	return fmt.Sprintf("%v", f.Interface())
}

// formFieldFromStruct produces an adminDBFormField from a struct field value.
func formFieldFromStruct(rec any, fieldName string) (adminDBFormField, bool) {
	v := reflect.ValueOf(rec)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	sf, ok := t.FieldByName(fieldName)
	if !ok {
		return adminDBFormField{}, false
	}
	f := v.FieldByName(fieldName)

	ff := adminDBFormField{Name: fieldName, Label: fieldName}
	// Check pointer
	innerKind := sf.Type.Kind()
	innerType := sf.Type
	if innerKind == reflect.Ptr {
		ff.IsNullable = true
		innerType = sf.Type.Elem()
		innerKind = innerType.Kind()
		if !f.IsNil() {
			f = f.Elem()
		} else {
			f = reflect.Value{}
		}
	}

	// time.Time?
	if innerType == reflect.TypeOf(time.Time{}) {
		ff.InputType = "datetime-local"
		if f.IsValid() {
			t := f.Interface().(time.Time)
			if !t.IsZero() {
				ff.StringValue = t.Format("2006-01-02T15:04")
			}
		}
		return ff, true
	}

	switch innerKind {
	case reflect.Bool:
		ff.InputType = "checkbox"
		if f.IsValid() && f.Bool() {
			ff.IsChecked = true
		}
	case reflect.String:
		ff.InputType = "text"
		if f.IsValid() {
			ff.StringValue = f.String()
		}
		// textarea for long description-like fields
		switch fieldName {
		case "Description", "TxtIntro", "TxtHome", "TxtDistrib", "Body", "Quantities", "Rights":
			ff.InputType = "textarea"
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ff.InputType = "number"
		if f.IsValid() {
			ff.StringValue = strconv.FormatInt(f.Int(), 10)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ff.InputType = "number"
		if f.IsValid() {
			ff.StringValue = strconv.FormatUint(f.Uint(), 10)
		}
	case reflect.Float32, reflect.Float64:
		ff.InputType = "number"
		if f.IsValid() {
			ff.StringValue = strconv.FormatFloat(f.Float(), 'f', -1, 64)
		}
	default:
		ff.InputType = "text"
	}
	return ff, true
}

// applyFormToStruct reads form values for each allowed field and sets them on rec.
// Only fields listed in allowed are touched. Type-aware.
func applyFormToStruct(c *gin.Context, rec any, allowed []string) error {
	v := reflect.ValueOf(rec)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("rec must be a pointer")
	}
	v = v.Elem()
	t := v.Type()

	for _, fname := range allowed {
		sf, ok := t.FieldByName(fname)
		if !ok {
			continue
		}
		f := v.FieldByName(fname)
		if !f.CanSet() {
			continue
		}

		innerType := sf.Type
		isPtr := innerType.Kind() == reflect.Ptr
		if isPtr {
			innerType = innerType.Elem()
		}

		// time.Time
		if innerType == reflect.TypeOf(time.Time{}) {
			raw := strings.TrimSpace(c.PostForm(fname))
			if raw == "" {
				if isPtr {
					f.Set(reflect.Zero(sf.Type))
				}
				continue
			}
			var parsed time.Time
			for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"} {
				if p, err := time.Parse(layout, raw); err == nil {
					parsed = p
					break
				}
			}
			if parsed.IsZero() {
				return fmt.Errorf("date invalide pour %s", fname)
			}
			if isPtr {
				f.Set(reflect.ValueOf(&parsed))
			} else {
				f.Set(reflect.ValueOf(parsed))
			}
			continue
		}

		kind := innerType.Kind()

		// Booleans: checkbox — key present means true, absent means false
		if kind == reflect.Bool {
			_, present := c.GetPostForm(fname)
			val := present
			if isPtr {
				f.Set(reflect.ValueOf(&val))
			} else {
				f.SetBool(val)
			}
			continue
		}

		raw := strings.TrimSpace(c.PostForm(fname))

		// Empty value on pointer → nil
		if isPtr && raw == "" {
			f.Set(reflect.Zero(sf.Type))
			continue
		}

		switch kind {
		case reflect.String:
			sv := reflect.New(innerType).Elem()
			sv.SetString(raw)
			if isPtr {
				p := reflect.New(innerType)
				p.Elem().Set(sv)
				f.Set(p)
			} else {
				f.Set(sv)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if raw == "" {
				// leave scalar zero
				continue
			}
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return fmt.Errorf("entier invalide pour %s", fname)
			}
			sv := reflect.New(innerType).Elem()
			sv.SetInt(n)
			if isPtr {
				p := reflect.New(innerType)
				p.Elem().Set(sv)
				f.Set(p)
			} else {
				f.Set(sv)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if raw == "" {
				continue
			}
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return fmt.Errorf("entier invalide pour %s", fname)
			}
			sv := reflect.New(innerType).Elem()
			sv.SetUint(n)
			if isPtr {
				p := reflect.New(innerType)
				p.Elem().Set(sv)
				f.Set(p)
			} else {
				f.Set(sv)
			}
		case reflect.Float32, reflect.Float64:
			if raw == "" {
				continue
			}
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return fmt.Errorf("nombre invalide pour %s", fname)
			}
			sv := reflect.New(innerType).Elem()
			sv.SetFloat(n)
			if isPtr {
				p := reflect.New(innerType)
				p.Elem().Set(sv)
				f.Set(p)
			} else {
				f.Set(sv)
			}
		}
	}
	return nil
}
