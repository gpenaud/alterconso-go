package handler

import (
	"fmt"
	"html/template"
	"net/http"
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

var funcMap = template.FuncMap{
	"not": func(v bool) bool { return !v },
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"nl2br": func(s *string) template.HTML {
		if s == nil {
			return ""
		}
		return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(*s), "\n", "<br>"))
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
	"hasFlag":    func(flags uint, f uint) bool { return flags&f != 0 },
	"derefUint":  func(i *uint) string {
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
	"paginateInts": paginateInts,
	"seq": func(start, end int) []int {
		s := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			s = append(s, i)
		}
		return s
	},
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
	Title          string
	User           *model.User
	Group          *model.Group
	IsGroupManager bool
	Category       string
	Breadcrumb     []BreadcrumbItem
	Flash          string
	FlashError     bool
	Redirect       string
	Container      string
	HideNav        bool
	// home page
	Groups        []model.Group
	MultiDistribs []MultiDistribView
	OpenCatalogs  []model.Catalog
	// contract_view page
	Catalog      *model.Catalog
	Products     []model.Product
	ProductViews []ProductView
	Vendor   *model.Vendor
	Contact  *model.User
	Distribs []DistribView
	// shop page
	MultiDistribID uint
	// account page
	Subscriptions []SubscriptionView
	RecentOrders  []RecentOrderView
	// member page
	Members []MemberView
	// distribution page (reuses MultiDistribs above but also:)
	AllDistribs []DistribAdminView
	PeriodLabel string
	// amapadmin page
	Places  []model.Place
	Admins  []model.User
	// amap page
	Vendors     []model.Vendor
	AmapVendors []AmapVendorView
	// contractAdmin page
	AdminCatalogs []CatalogAdminRow
	// account page membership
	MembershipRenewalPeriod string
	// member page pagination
	TotalMembers     int
	TotalPages       int
	CurrentPage      int
	WaitingListCount int
}

type MultiDistribView struct {
	ID              uint
	Place           string
	PlaceAddress    string
	DayOfWeek       string
	Day             string
	Month           string
	StartHour       string
	EndHour         string
	Active          bool
	Past            bool
	CanOrder        bool
	OrderNotYetOpen bool
	OrderStartDate  string
	OrderEndDate    string
	Distributions   bool
	UserOrders      []UserOrderView
	ProductImages   []ProductImageView
	VolunteerNeeded int
	VolunteerRoles  []string
}

type UserOrderView struct {
	ProductName string
	SmartQty    string
}

type ProductImageView struct {
	URL  string
	Name string
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
	SmartQty    string
	Total       float64
	Paid        bool
}

type MemberView struct {
	ID        uint
	FirstName string
	LastName  string
	Email     string
	Balance   float64
	IsManager bool
	Address   string
}

type DistribAdminView struct {
	ID                  uint
	DayOfWeek           string
	Day                 string
	Month               string
	Date                string
	DateISO             string // YYYY-MM-DD for URL
	StartHour           string
	EndHour             string
	Place               string
	PlaceAddress        string
	OrderStartDate      string
	OrderEndDate        string
	Catalogs            []string
	DistribLinks        []DistribLink
	Validated           bool
	NbOrders            int
	NbVolunteers        int
	NbVolunteersRequired int
	TotalAmount         float64
	IsFuture            bool
	IsOrderOpen         bool
	IsPast              bool
	IsToday             bool
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
	DistribID   uint
	CatalogID   uint
	CatalogName string
	VendorName  string
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
func (h *PagesHandler) buildPageData(c *gin.Context) PageData {
	pd := PageData{}
	claims := middleware.GetClaims(c)
	if claims == nil {
		return pd
	}

	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err == nil {
		pd.User = &user
	}

	if claims.GroupID != 0 {
		var group model.Group
		if err := h.db.First(&group, claims.GroupID).Error; err == nil {
			pd.Group = &group
		}
		// Check manager right
		var ug model.UserGroup
		if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, claims.GroupID).
			First(&ug).Error; err == nil {
			pd.IsGroupManager = ug.IsGroupManager()
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
	t, err := loadTemplates("base.html", "login.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	pd := PageData{Title: "Connexion", Redirect: redirect}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Logout ----

func (h *PagesHandler) Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/user/login")
}

// ---- Group selection ----

func (h *PagesHandler) ChoosePage(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	// If ?group=id → switch group and redirect to /home
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		var groupID uint
		if _, err := fmt.Sscanf(groupIDStr, "%d", &groupID); err == nil && groupID != 0 {
			// Verify user is member of this group
			var ug model.UserGroup
			if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).
				First(&ug).Error; err == nil {
				// Issue new JWT with GroupID set
				newToken, err := h.issueToken(claims.UserID, groupID)
				if err == nil {
					c.SetCookie("token", newToken, 3600*24*7, "/", "", false, true)
					c.Redirect(http.StatusFound, "/home")
					return
				}
			}
		}
	}

	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	// Load user's groups
	var ugs []model.UserGroup
	h.db.Where("user_id = ?", claims.UserID).Find(&ugs)
	groupIDs := make([]uint, 0, len(ugs))
	for _, ug := range ugs {
		groupIDs = append(groupIDs, ug.GroupID)
	}

	var groups []model.Group
	if len(groupIDs) > 0 {
		h.db.Where("id IN ?", groupIDs).Find(&groups)
	}

	t, err := loadTemplates("base.html", "design.html", "choose.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	pd := PageData{
		Title:   "Choisir un groupe",
		User:    &user,
		Groups:  groups,
		HideNav: true,
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Home page ----

func (h *PagesHandler) HomePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login?__redirect=/home")
		return
	}
	if pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	pd.Title = "Accueil"
	pd.Category = "home"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Commandes", Link: "/home"}}

	claims := middleware.GetClaims(c)

	// Period navigation
	offsetStr := c.DefaultQuery("offset", "0")
	offsetWeeks, _ := strconv.Atoi(offsetStr)

	frMonthsFull := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	frDaysFull := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frDays := [...]string{"Dim", "Lun", "Mar", "Mer", "Jeu", "Ven", "Sam"}

	now := time.Now()
	// 2-week window starting on last Saturday
	weekday := int(now.Weekday()) // 0=Sun
	daysSinceSat := (weekday + 1) % 7
	periodStart := now.AddDate(0, 0, -daysSinceSat+offsetWeeks*14)
	periodStart = time.Date(periodStart.Year(), periodStart.Month(), periodStart.Day(), 0, 0, 0, 0, periodStart.Location())
	periodEnd := periodStart.AddDate(0, 0, 14)
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
			Active:       now.After(start) && now.Before(end),
			Past:         !now.Before(time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())),
		}

		// Product images from all catalogs in this distribution (max 8)
		for _, d := range md.Distributions {
			if len(view.ProductImages) >= 8 {
				break
			}
			remaining := 8 - len(view.ProductImages)
			var prods []model.Product
			h.db.Where("catalog_id = ?", d.Catalog.ID).
				Preload("Image").Limit(remaining).Find(&prods)
			for _, p := range prods {
				url := "/img/taxo/grey/fruits-legumes.png"
				if p.Image != nil {
					url = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
				}
				view.ProductImages = append(view.ProductImages, ProductImageView{URL: url, Name: p.Name})
			}
		}

		// Determine order state from first distribution
		if len(md.Distributions) > 0 {
			view.Distributions = true
			d := md.Distributions[0]
			orderEnd := md.OrderEndDate
			orderStart := md.OrderStartDate
			if orderEnd == nil {
				orderEnd = d.OrderEndDate
				orderStart = d.OrderStartDate
			}
			if orderEnd == nil {
				view.CanOrder = d.Catalog.UsersCanOrder()
			} else {
				if orderStart != nil && now.Before(*orderStart) {
					view.OrderNotYetOpen = true
					view.OrderStartDate = fmt.Sprintf("%s %d %s à %02d:%02d",
						frDays[orderStart.Weekday()], orderStart.Day(),
						frMonthsFull[orderStart.Month()], orderStart.Hour(), orderStart.Minute())
				} else if now.Before(*orderEnd) {
					view.CanOrder = true
					view.OrderEndDate = fmt.Sprintf("%s %d %s à %02d:%02d",
						frDays[orderEnd.Weekday()], orderEnd.Day(),
						frMonthsFull[orderEnd.Month()], orderEnd.Hour(), orderEnd.Minute())
				}
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
		nbNeeded := len(rolesNeeded)
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
			view.UserOrders = append(view.UserOrders, UserOrderView{
				ProductName: o.Product.Name,
				SmartQty:    formatQty(o.Quantity, o.Product.UnitType),
			})
		}

		views = append(views, view)
	}
	pd.MultiDistribs = views

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

	t, err := loadTemplates("base.html", "design.html", "home.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
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
		if p.Ref != nil { ref = *p.Ref }
		qt := 0.0
		if p.Qt != nil {
			qt = *p.Qt
		}
		productViews = append(productViews, ProductView{
			ID:       p.ID,
			Name:     p.Name,
			Ref:      ref,
			UnitType: unitLabels[p.UnitType],
			Price:    p.Price,
			VAT:      p.VAT,
			Qt:       qt,
			Organic:  p.Organic,
			ImageURL: imgURL,
		})
	}

	pd.Title = catalog.Name
	pd.Catalog = &catalog
	pd.Products = catalog.Products
	pd.ProductViews = productViews
	pd.Vendor = &catalog.Vendor
	pd.Contact = catalog.Contact
	pd.Distribs = distribViews

	t, err := loadTemplates("base.html", "design.html", "contract_view.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- Shop page ----

func (h *PagesHandler) ShopPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	id, err := strconv.ParseUint(c.Param("multiDistribId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	pd.Title = "Boutique"
	pd.MultiDistribID = uint(id)

	t, err := loadTemplates("base.html", "design.html", "shop.html")
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
		if o.Product.Catalog.Type != model.CatalogTypeVarOrder {
			continue
		}
		pd.RecentOrders = append(pd.RecentOrders, RecentOrderView{
			ProductName: o.Product.Name,
			SmartQty:    formatQty(o.Quantity, o.Product.UnitType),
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
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Mon compte", Link: "/account"}}

	t, err := loadTemplates("base.html", "design.html", "account.html")
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
	pd.Breadcrumb = []BreadcrumbItem{
		{Name: "Mon compte", Link: "/account"},
		{Name: "Modifier", Link: ""},
	}
	t, err := loadTemplates("base.html", "design.html", "account_edit.html")
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

func (h *PagesHandler) MemberPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	if !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	const perPage = 10
	pageStr := c.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	var totalCount int64
	h.db.Model(&model.UserGroup{}).Where("group_id = ?", pd.Group.ID).Count(&totalCount)
	totalPages := int(totalCount) / perPage
	if int(totalCount)%perPage != 0 {
		totalPages++
	}

	var ugs []model.UserGroup
	h.db.Where("group_id = ?", pd.Group.ID).Preload("User").
		Offset((page - 1) * perPage).Limit(perPage).Find(&ugs)

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
		pd.Members = append(pd.Members, MemberView{
			ID:        ug.User.ID,
			FirstName: ug.User.FirstName,
			LastName:  ug.User.LastName,
			Email:     ug.User.Email,
			Balance:   ug.Balance,
			IsManager: ug.IsGroupManager(),
			Address:   addr,
		})
	}

	pd.TotalMembers = int(totalCount)
	pd.TotalPages = totalPages
	pd.CurrentPage = page

	var waitingCount int64
	h.db.Model(&model.WaitingList{}).
		Joins("JOIN catalogs ON catalogs.id = waiting_lists.catalog_id").
		Where("catalogs.group_id = ?", pd.Group.ID).
		Count(&waitingCount)
	pd.WaitingListCount = int(waitingCount)

	pd.Title = "Membres"
	pd.Category = "member"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Membres", Link: "/member"}}

	t, err := loadTemplates("base.html", "design.html", "member.html")
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
	if !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	// Period navigation
	offsetStr := c.DefaultQuery("offset", "0")
	offsetWeeks, _ := strconv.Atoi(offsetStr)

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
		for _, d := range md.Distributions {
			catalogs = append(catalogs, d.Catalog.Name)
			links = append(links, DistribLink{
				DistribID:   d.ID,
				CatalogID:   d.CatalogID,
				CatalogName: d.Catalog.Name,
				VendorName:  d.Catalog.Vendor.Name,
			})
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
			ID:                  md.ID,
			DayOfWeek:           frDaysFull[md.DistribStartDate.Weekday()],
			Day:                 fmt.Sprintf("%d", md.DistribStartDate.Day()),
			Month:               frMonthsFull[md.DistribStartDate.Month()],
			Date:                md.DistribStartDate.Format("02/01/2006"),
			DateISO:             md.DistribStartDate.Format("2006-01-02"),
			StartHour:           md.DistribStartDate.Format("15:04"),
			EndHour:             md.DistribEndDate.Format("15:04"),
			Place:               md.Place.Name,
			PlaceAddress:        placeAddr,
			OrderStartDate:      orderStartStr,
			OrderEndDate:        orderEndStr,
			Catalogs:            catalogs,
			DistribLinks:        links,
			Validated:           md.Validated,
			NbOrders:            int(nbOrders),
			NbVolunteers:        int(nbVols),
			NbVolunteersRequired: int(nbVolRoles),
			TotalAmount:         totalAmt,
			IsFuture:            md.DistribStartDate.After(now),
			IsPast:              md.DistribStartDate.Before(now),
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

	t, err := loadTemplates("base.html", "design.html", "distribution.html")
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
			if v.City != nil { city = *v.City }
			if v.ZipCode != nil { zip = *v.ZipCode }
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

	t, err := loadTemplates("base.html", "design.html", "amap.html")
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
	if !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	frDays := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frMonths := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	frDate := func(t *time.Time, withTime bool) string {
		if t == nil {
			return ""
		}
		s := fmt.Sprintf("%s %d %s", frDays[t.Weekday()], t.Day(), frMonths[t.Month()])
		if withTime && (t.Hour() != 0 || t.Minute() != 0) {
			s += fmt.Sprintf(" à %02d:%02d", t.Hour(), t.Minute())
		}
		return s
	}

	var catalogs []model.Catalog
	h.db.Where("group_id = ?", pd.Group.ID).
		Preload("Vendor").
		Order("name ASC").
		Find(&catalogs)

	for _, cat := range catalogs {
		startStr := ""
		endStr := ""
		if cat.StartDate != nil {
			startStr = "du " + frDate(cat.StartDate, false)
		}
		if cat.EndDate != nil {
			endStr = "au " + frDate(cat.EndDate, true)
		}
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

	t, err := loadTemplates("base.html", "design.html", "contract_admin.html")
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
	if !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	// Load places for the group
	h.db.Where("group_id = ?", pd.Group.ID).Find(&pd.Places)

	// Load admins for contact/legal selects
	h.db.Joins("JOIN user_groups ON user_groups.user_id = users.id").
		Where("user_groups.group_id = ? AND user_groups.rights LIKE ?", pd.Group.ID, "%GroupAdmin%").
		Order("users.last_name").Find(&pd.Admins)

	pd.Title = "Paramètres"
	pd.Category = "amapadmin"
	pd.Breadcrumb = []BreadcrumbItem{{Name: "Paramètres", Link: "/amapadmin"}}

	t, err := loadTemplates("base.html", "design.html", "amapadmin.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- AmapAdmin update ----

func (h *PagesHandler) AmapAdminUpdate(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	name := c.PostForm("name")
	txtIntro := c.PostForm("txt_intro")
	txtHome := c.PostForm("txt_home")
	txtDistrib := c.PostForm("txt_distrib")
	extURL := c.PostForm("ext_url")
	groupType := c.PostForm("group_type")
	regOption := c.PostForm("reg_option")

	// Flags
	var flags uint
	if c.PostForm("flag_payments") == "1"            { flags |= uint(model.GroupFlagHasPayments) }
	if c.PostForm("flag_network") == "1"             { flags |= uint(model.GroupFlagCagetteNetwork) }
	if c.PostForm("flag_custom_categories") == "1"   { flags |= uint(model.GroupFlagCustomizedCategories) }
	if c.PostForm("flag_hide_phone") == "1"          { flags |= uint(model.GroupFlagHidePhone) }
	if c.PostForm("flag_phone_required") == "1"      { flags |= uint(model.GroupFlagPhoneRequired) }
	if c.PostForm("flag_address_required") == "1"    { flags |= uint(model.GroupFlagAddressRequired) }
	if c.PostForm("flag_shop_mode") == "1"           { flags |= uint(model.GroupFlagShopMode) }

	updates := map[string]interface{}{
		"name":       name,
		"group_type": groupType,
		"reg_option": regOption,
		"flags":      flags,
	}
	if txtIntro != "" { updates["txt_intro"] = txtIntro } else { updates["txt_intro"] = nil }
	if txtHome != "" { updates["txt_home"] = txtHome } else { updates["txt_home"] = nil }
	if txtDistrib != "" { updates["txt_distrib"] = txtDistrib } else { updates["txt_distrib"] = nil }
	if extURL != "" { updates["ext_url"] = extURL } else { updates["ext_url"] = nil }

	if cid, err := strconv.ParseUint(c.PostForm("contact_id"), 10, 64); err == nil && cid > 0 {
		updates["contact_id"] = uint(cid)
	} else {
		updates["contact_id"] = nil
	}
	if lid, err := strconv.ParseUint(c.PostForm("legal_representative_id"), 10, 64); err == nil && lid > 0 {
		updates["legal_representative_id"] = uint(lid)
	} else {
		updates["legal_representative_id"] = nil
	}

	h.db.Model(&model.Group{}).Where("id = ?", pd.Group.ID).Updates(updates)
	c.Redirect(http.StatusFound, "/amapadmin")
}

// ---- Helpers ----

func (h *PagesHandler) issueToken(userID, groupID uint) (string, error) {
	claims := &middleware.Claims{
		UserID:  userID,
		GroupID: groupID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

func formatQty(qty float64, unit model.UnitType) string {
	switch unit {
	case model.UnitTypeKilogram:
		if qty < 1 {
			return fmt.Sprintf("%.0fg", qty*1000)
		}
		if qty == float64(int(qty)) {
			return fmt.Sprintf("%.0fkg", qty)
		}
		return fmt.Sprintf("%.2fkg", qty)
	case model.UnitTypeGram:
		return fmt.Sprintf("%.0fg", qty)
	case model.UnitTypeLitre:
		if qty == float64(int(qty)) {
			return fmt.Sprintf("%.0fL", qty)
		}
		return fmt.Sprintf("%.2fL", qty)
	default:
		if qty == float64(int(qty)) {
			return fmt.Sprintf("%.0f", qty)
		}
		return fmt.Sprintf("%.2f", qty)
	}
}
