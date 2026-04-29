package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
)

// floatToFractionStr convertit un float en chaîne fractionnaire lisible.
func floatToFractionStr(v float64) string {
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
}

// parseFraction parse "1/4", "1/2", "0.5", "1" etc. en float64.
func parseFraction(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		num, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		den, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0, fmt.Errorf("invalid fraction")
		}
		return num / den, nil
	}
	return strconv.ParseFloat(s, 64)
}

// ---- Common contractAdmin data ----

type ProductView struct {
	ID            uint
	Name          string
	Ref           string
	UnitType      string
	QtLabel       string // ex: "1 pièce", "500 g"
	Price         float64
	PriceLabel    string // ex: "2,20 €"
	VAT           float64
	Qt            float64
	Organic       bool
	VariablePrice bool
	Active        bool
	Stock         float64
	StockTracked  bool
	ImageURL      string
}

type CatalogAdminData struct {
	PageData
	Catalog   model.Catalog
	ActiveTab string
	ShowOld      bool
	AllJoined    bool
	// per-tab content
	Products      []model.Product
	ProductViews  []ProductView
	CatalogDistribs []CatalogDistribEntry
	Orders        []CatalogOrderEntry
	Subscriptions []CatalogSubEntry
}

type CatalogDistribEntry struct {
	DistribID      uint
	MultiDistribID uint
	Date           string
	DateLabel      string // "Vendredi 10 Avril à 17:00"
	StartHour      string
	EndHour        string
	Place          string
	OrdersOpen     bool
	NbOrders       int
	Participating  bool
	IsPast         bool
}

type CatalogOrderEntry struct {
	UserID      uint
	UserName    string
	BasketNum   int
	Lines       []OrderLineView
	Total       float64
}

type CatalogSubEntry struct {
	SubID       uint
	UserID      uint
	UserName    string
	StartDate   string
	EndDate     string
	Validated   bool
}

// ---- /contractAdmin/view/:id ----

func (h *PagesHandler) CatalogAdminViewPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "view")
	if !ok {
		return
	}
	var products []model.Product
	h.db.Where("catalog_id = ?", data.Catalog.ID).Preload("Image").Find(&products)
	data.Products = products
	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_view.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/edit/:id ----

type CatalogEditData struct {
	CatalogAdminData
	Vendors []model.Vendor
	Members []model.User
}

func (h *PagesHandler) CatalogAdminEditPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "view")
	if !ok {
		return
	}

	if c.Request.Method == "POST" {
		cat := &data.Catalog
		cat.Name = c.PostForm("name")
		if sd := c.PostForm("start_date"); sd != "" {
			if t, err := time.Parse("2006-01-02", sd); err == nil {
				cat.StartDate = &t
			}
		} else {
			cat.StartDate = nil
		}
		if ed := c.PostForm("end_date"); ed != "" {
			if t, err := time.Parse("2006-01-02", ed); err == nil {
				cat.EndDate = &t
			}
		} else {
			cat.EndDate = nil
		}
		// Flags
		cat.Flags = 0
		if c.PostForm("users_can_order") == "1" {
			cat.SetFlag(model.CatalogFlagUsersCanOrder)
		}
		if c.PostForm("stock_management") == "1" {
			cat.SetFlag(model.CatalogFlagStockManagement)
		}
		if c.PostForm("percentage_fees") == "1" {
			cat.SetFlag(model.CatalogFlagHasPercentageFees)
			if pf := c.PostForm("percentage_fees_value"); pf != "" {
				if v, err := strconv.ParseFloat(pf, 64); err == nil {
					cat.PercentageFees = &v
				}
			}
			if pn := c.PostForm("percentage_name"); pn != "" {
				cat.PercentageName = &pn
			}
		} else {
			cat.PercentageFees = nil
			cat.PercentageName = nil
		}
		if vid, err := strconv.ParseUint(c.PostForm("vendor_id"), 10, 64); err == nil {
			cat.VendorID = uint(vid)
		}
		if cid := c.PostForm("contact_id"); cid != "" {
			if v, err := strconv.ParseUint(cid, 10, 64); err == nil {
				uid := uint(v)
				cat.ContactID = &uid
			}
		} else {
			cat.ContactID = nil
		}
		updates := map[string]interface{}{
			"name":            cat.Name,
			"flags":           cat.Flags,
			"start_date":      cat.StartDate,
			"end_date":        cat.EndDate,
			"vendor_id":       cat.VendorID,
			"contact_id":      cat.ContactID,
			"percentage_fees": cat.PercentageFees,
			"percentage_name": cat.PercentageName,
		}
		if err := h.db.Model(&model.Catalog{}).Where("id = ?", cat.ID).Updates(updates).Error; err != nil {
			c.String(http.StatusInternalServerError, "erreur sauvegarde: %v", err)
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/view/%d", cat.ID))
		return
	}

	var vendors []model.Vendor
	h.db.Order("name").Find(&vendors)
	var members []model.User
	h.db.Joins("JOIN user_groups ON user_groups.user_id = users.id").
		Where("user_groups.group_id = ? AND user_groups.rights LIKE ?", data.Group.ID, "%GroupAdmin%").
		Order("users.last_name").Find(&members)

	editData := CatalogEditData{CatalogAdminData: data, Vendors: vendors, Members: members}
	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", editData); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/products/:id ----

func (h *PagesHandler) CatalogAdminProductsPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "products")
	if !ok {
		return
	}

	var products []model.Product
	h.db.Where("catalog_id = ?", data.Catalog.ID).Preload("Image").Find(&products)
	data.Products = products

	unitLabels := map[model.UnitType]string{
		model.UnitTypePiece:      "pièce",
		"Unit":                   "pièce", // alias legacy
		model.UnitTypeKilogram:   "kg",
		model.UnitTypeGram:       "g",
		model.UnitTypeLitre:      "L",
		model.UnitTypeCentilitre: "cl",
		model.UnitTypeMillilitre: "ml",
	}
	for _, p := range products {
		imgURL := "/img/taxo/grey/fruits-legumes.png"
		if p.Image != nil {
			imgURL = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
		}
		ref := ""
		if p.Ref != nil {
			ref = *p.Ref
		}
		stock := 0.0
		if p.Stock != nil {
			stock = *p.Stock
		}
		unit := unitLabels[p.UnitType]
		if unit == "" {
			unit = "pièce"
		}
		qt := 1.0
		if p.Qt != nil && *p.Qt != 0 {
			qt = *p.Qt
		}
		qtLabel := fmt.Sprintf("%s %s", floatToFractionStr(qt), unit)
		priceLabel := fmt.Sprintf("%.2f €", p.Price)
		data.ProductViews = append(data.ProductViews, ProductView{
			ID:            p.ID,
			Name:          p.Name,
			Ref:           ref,
			UnitType:      unit,
			QtLabel:       qtLabel,
			Price:         p.Price,
			PriceLabel:    priceLabel,
			Organic:       p.Organic,
			VariablePrice: p.VariablePrice,
			Active:        p.Active,
			Stock:         stock,
			StockTracked:  p.StockTracked,
			ImageURL:      imgURL,
		})
	}

	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_products.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/products/:catalogId/edit/:productId ----

type ProductEditData struct {
	CatalogAdminData
	Product  model.Product
	ImageURL string
}

func (h *PagesHandler) CatalogAdminProductEditPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "products")
	if !ok {
		return
	}
	pid, err := strconv.ParseUint(c.Param("productId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id produit invalide")
		return
	}
	var product model.Product
	if err := h.db.Preload("Image").First(&product, pid).Error; err != nil || product.CatalogID != data.Catalog.ID {
		c.String(http.StatusNotFound, "produit introuvable")
		return
	}

	if c.Request.Method == "POST" {
		product.Name = c.PostForm("name")
		if ref := c.PostForm("ref"); ref != "" {
			product.Ref = &ref
		} else {
			product.Ref = nil
		}
		if desc := c.PostForm("description"); desc != "" {
			product.Description = &desc
		} else {
			product.Description = nil
		}
		if p, err := strconv.ParseFloat(c.PostForm("price"), 64); err == nil {
			product.Price = p
		}
		if v, err := strconv.ParseFloat(c.PostForm("vat"), 64); err == nil {
			product.VAT = v
		}
		if qt, err := parseFraction(c.PostForm("qt")); err == nil {
			product.Qt = &qt
		} else {
			product.Qt = nil
		}
		product.UnitType = model.UnitType(c.PostForm("unit_type"))
		product.Organic = c.PostForm("organic") == "1"
		product.VariablePrice = c.PostForm("variable_price") == "1"
		product.MultiWeight = c.PostForm("multi_weight") == "1"
		product.HasFloatQt = c.PostForm("has_float_qt") == "1"
		product.Active = c.PostForm("active") == "1"
		product.IsResale = c.PostForm("is_resale") == "1"
		if rf := strings.TrimSpace(c.PostForm("resale_from")); rf != "" && product.IsResale {
			product.ResaleFrom = &rf
		} else {
			product.ResaleFrom = nil
		}
		h.db.Model(&model.Product{}).Where("id = ?", product.ID).Updates(map[string]interface{}{
			"name":           product.Name,
			"ref":            product.Ref,
			"description":    product.Description,
			"qt":             product.Qt,
			"price":          product.Price,
			"vat":            product.VAT,
			"unit_type":      product.UnitType,
			"organic":        product.Organic,
			"variable_price": product.VariablePrice,
			"multi_weight":   product.MultiWeight,
			"has_float_qt":   product.HasFloatQt,
			"active":         product.Active,
			"is_resale":      product.IsResale,
			"resale_from":    product.ResaleFrom,
		})
		c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/products/%d", data.Catalog.ID))
		return
	}

	imgURL := ""
	if product.Image != nil {
		imgURL = FileURL(product.Image.ID, h.cfg.Key, product.Image.Name)
	}
	editData := ProductEditData{CatalogAdminData: data, Product: product, ImageURL: imgURL}
	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_product_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", editData); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/products/:catalogId/photo/:productId ----

func (h *PagesHandler) CatalogAdminProductPhotoPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "products")
	if !ok {
		return
	}
	pid, err := strconv.ParseUint(c.Param("productId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id produit invalide")
		return
	}
	var product model.Product
	if err := h.db.First(&product, pid).Error; err != nil || product.CatalogID != data.Catalog.ID {
		c.String(http.StatusNotFound, "produit introuvable")
		return
	}

	imgURL := ""
	if product.Image != nil {
		imgURL = FileURL(product.Image.ID, h.cfg.Key, product.Image.Name)
	}

	if c.Request.Method == "POST" {
		file, err := c.FormFile("photo")
		if err != nil {
			c.String(http.StatusBadRequest, "fichier manquant: %v", err)
			return
		}
		f, err := file.Open()
		if err != nil {
			c.String(http.StatusInternalServerError, "erreur ouverture fichier: %v", err)
			return
		}
		defer f.Close()
		buf, err := io.ReadAll(f)
		if err != nil {
			c.String(http.StatusInternalServerError, "erreur lecture fichier: %v", err)
			return
		}

		dbFile := model.File{Name: file.Filename, Data: buf}
		if err := h.db.Create(&dbFile).Error; err != nil {
			c.String(http.StatusInternalServerError, "erreur sauvegarde fichier: %v", err)
			return
		}

		if product.ImageID != nil {
			h.db.Delete(&model.File{}, *product.ImageID)
		}
		if err := h.db.Model(&model.Product{}).Where("id = ?", product.ID).Update("imageId", dbFile.ID).Error; err != nil {
			c.String(http.StatusInternalServerError, "erreur mise à jour produit: %v", err)
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/products/%d", data.Catalog.ID))
		return
	}

	editData := ProductEditData{CatalogAdminData: data, Product: product, ImageURL: imgURL}
	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_product_photo.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", editData); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/products/:catalogId/delete/:productId ----

func (h *PagesHandler) CatalogAdminProductDeletePage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "products")
	if !ok {
		return
	}
	pid, err := strconv.ParseUint(c.Param("productId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id produit invalide")
		return
	}
	var product model.Product
	if err := h.db.First(&product, pid).Error; err != nil || product.CatalogID != data.Catalog.ID {
		c.String(http.StatusNotFound, "produit introuvable")
		return
	}
	if product.ImageID != nil {
		h.db.Delete(&model.File{}, *product.ImageID)
	}
	h.db.Delete(&model.Product{}, pid)
	c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/products/%d", data.Catalog.ID))
}

// ---- POST /contractAdmin/products/:id/bulkAction ----

func (h *PagesHandler) CatalogAdminProductsBulkAction(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "products")
	if !ok {
		return
	}
	action := c.PostForm("action")
	ids := c.PostFormArray("product_ids[]")
	active := action == "activate"
	for _, idStr := range ids {
		pid, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		h.db.Model(&model.Product{}).Where("id = ? AND catalog_id = ?", pid, data.Catalog.ID).
			Update("active", active)
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/products/%d", data.Catalog.ID))
}

// ---- /contractAdmin/distributions/:id ----

func (h *PagesHandler) CatalogAdminDistributionsPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "distributions")
	if !ok {
		return
	}

	// Handle "join all" action
	if c.Request.Method == http.MethodPost {
		action := c.PostForm("action")
		if action == "joinAll" {
			var allMDs []model.MultiDistrib
			h.db.Where("group_id = ? AND distrib_start_date >= ?", data.Catalog.GroupID, time.Now()).Find(&allMDs)
			for _, md := range allMDs {
				var existing model.Distribution
				err := h.db.Where("catalog_id = ? AND multi_distrib_id = ?", data.Catalog.ID, md.ID).First(&existing).Error
				if err != nil {
					h.db.Create(&model.Distribution{CatalogID: data.Catalog.ID, MultiDistribID: md.ID})
				}
			}
		} else if action == "leaveAll" {
			var futureDistribs []model.Distribution
			h.db.Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
				Where("distributions.catalog_id = ? AND multi_distribs.distrib_start_date >= ?", data.Catalog.ID, time.Now()).
				Find(&futureDistribs)
			for _, d := range futureDistribs {
				h.db.Delete(&model.Distribution{}, d.ID)
			}
		} else if action == "join" {
			mdIDStr := c.PostForm("multi_distrib_id")
			mdID, err := strconv.ParseUint(mdIDStr, 10, 64)
			if err == nil {
				var existing model.Distribution
				err2 := h.db.Where("catalog_id = ? AND multi_distrib_id = ?", data.Catalog.ID, mdID).First(&existing).Error
				if err2 != nil {
					h.db.Create(&model.Distribution{CatalogID: data.Catalog.ID, MultiDistribID: uint(mdID)})
				}
			}
		} else if action == "leave" {
			distribIDStr := c.PostForm("distrib_id")
			distribID, err := strconv.ParseUint(distribIDStr, 10, 64)
			if err == nil {
				h.db.Where("id = ? AND catalog_id = ?", distribID, data.Catalog.ID).Delete(&model.Distribution{})
			}
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/distributions/%d", data.Catalog.ID))
		return
	}

	now := time.Now()
	showOld := c.Query("old") == "1"
	data.ShowOld = showOld

	frDays := []string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frMonths := []string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}

	// All multi_distribs for the group
	var allMDs []model.MultiDistrib
	q := h.db.Where("group_id = ?", data.Catalog.GroupID).Preload("Place").Order("distrib_start_date ASC")
	if !showOld {
		q = q.Where("distrib_start_date >= ?", now.AddDate(0, 0, -1))
	}
	q.Find(&allMDs)

	// Existing distributions for this catalog
	var distribs []model.Distribution
	h.db.Where("catalog_id = ?", data.Catalog.ID).Find(&distribs)
	distribMap := map[uint]model.Distribution{}
	for _, d := range distribs {
		distribMap[d.MultiDistribID] = d
	}

	for _, md := range allMDs {
		d, participating := distribMap[md.ID]
		isPast := md.DistribStartDate.Before(now)
		dateLabel := frDays[md.DistribStartDate.Weekday()] + " " +
			strconv.Itoa(md.DistribStartDate.Day()) + " " +
			frMonths[md.DistribStartDate.Month()] + " " +
			strconv.Itoa(md.DistribStartDate.Year())

		var nb int64
		if participating {
			h.db.Model(&model.UserOrder{}).Where("distribution_id = ?", d.ID).Count(&nb)
		}

		entry := CatalogDistribEntry{
			MultiDistribID: md.ID,
			DateLabel:      dateLabel,
			Date:           md.DistribStartDate.Format("2006-01-02"),
			StartHour:      md.DistribStartDate.Format("15:04"),
			EndHour:        md.DistribEndDate.Format("15:04"),
			Place:          md.Place.Name,
			NbOrders:       int(nb),
			Participating:  participating,
			IsPast:         isPast,
		}
		if participating {
			entry.DistribID = d.ID
			entry.OrdersOpen = d.CanOrderNow()
		}
		data.CatalogDistribs = append(data.CatalogDistribs, entry)
	}

	// AllJoined = all future distributions are participated
	allJoined := len(data.CatalogDistribs) > 0
	for _, e := range data.CatalogDistribs {
		if !e.IsPast && !e.Participating {
			allJoined = false
			break
		}
	}
	data.AllJoined = allJoined

	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_distributions.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/orders/:id?d=<distribId> ----

func (h *PagesHandler) CatalogAdminOrdersPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "orders")
	if !ok {
		return
	}

	distribIDStr := c.Query("d")
	if distribIDStr == "" {
		// Redirect to distributions to select one
		c.Redirect(http.StatusFound, "/contractAdmin/distributions/"+c.Param("id"))
		return
	}

	distribID, _ := strconv.ParseUint(distribIDStr, 10, 64)
	var distrib model.Distribution
	if err := h.db.Preload("MultiDistrib").Preload("MultiDistrib.Place").
		First(&distrib, distribID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	var orders []model.UserOrder
	h.db.Where("distribution_id = ?", distribID).
		Preload("User").
		Preload("Product").
		Preload("Product.Catalog").
		Order("user_id").
		Find(&orders)

	// Group by user
	type userKey struct{ id uint }
	userMap := make(map[uint]*CatalogOrderEntry)
	userOrder := []uint{}

	for _, o := range orders {
		if _, ok := userMap[o.UserID]; !ok {
			userMap[o.UserID] = &CatalogOrderEntry{
				UserID:   o.UserID,
				UserName: o.User.FirstName + " " + o.User.LastName,
			}
			userOrder = append(userOrder, o.UserID)
		}
		fees := o.TotalPrice() - o.Quantity*o.ProductPrice
		line := OrderLineView{
			ProductName:  o.Product.Name,
			SmartQty:     formatQty(o.Quantity, o.Product.UnitType),
			ProductPrice: o.ProductPrice,
			SubTotal:     o.Quantity * o.ProductPrice,
			Fees:         fees,
			Total:        o.TotalPrice(),
			Paid:         o.Paid,
		}
		userMap[o.UserID].Lines = append(userMap[o.UserID].Lines, line)
		userMap[o.UserID].Total += o.TotalPrice()
	}

	for _, uid := range userOrder {
		data.Orders = append(data.Orders, *userMap[uid])
	}

	// Store distrib info in catalog for template access
	type OrdersData struct {
		CatalogAdminData
		Distrib    model.Distribution
		DistribDate string
		DistribPlace string
	}
	od := OrdersData{
		CatalogAdminData: data,
		Distrib:          distrib,
		DistribDate:      distrib.MultiDistrib.DistribStartDate.Format("02/01/2006"),
		DistribPlace:     distrib.MultiDistrib.Place.Name,
	}

	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_orders.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", od); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /contractAdmin/subscriptions/:id ----

func (h *PagesHandler) CatalogAdminSubscriptionsPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "subscriptions")
	if !ok {
		return
	}

	var subs []model.Subscription
	h.db.Where("catalog_id = ?", data.Catalog.ID).
		Preload("User").
		Order("created_at DESC").
		Find(&subs)

	for _, s := range subs {
		entry := CatalogSubEntry{
			SubID:     s.ID,
			UserID:    s.UserID,
			UserName:  s.User.FirstName + " " + s.User.LastName,
			StartDate: s.StartDate.Format("02/01/2006"),
		}
		if s.EndDate != nil {
			entry.EndDate = s.EndDate.Format("02/01/2006")
		}
		data.Subscriptions = append(data.Subscriptions, entry)
	}

	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_subscriptions.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- helper: load catalog + check permissions ----

func (h *PagesHandler) loadCatalogAdmin(c *gin.Context, tab string) (CatalogAdminData, bool) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasCatalogAdmin) {
		c.Redirect(http.StatusFound, "/home")
		return CatalogAdminData{}, false
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return CatalogAdminData{}, false
	}

	var catalog model.Catalog
	if err := h.db.Preload("Vendor").Preload("Contact").First(&catalog, id).Error; err != nil {
		c.String(http.StatusNotFound, "catalogue introuvable")
		return CatalogAdminData{}, false
	}

	if catalog.GroupID != pd.Group.ID || !pd.CanManageCatalog(catalog.ID) {
		c.Redirect(http.StatusFound, "/contractAdmin")
		return CatalogAdminData{}, false
	}

	data := CatalogAdminData{
		PageData:  pd,
		Catalog:   catalog,
		ActiveTab: tab,
	}
	data.Title = catalog.Name + " — " + catalog.Vendor.Name
	data.Breadcrumb = []BreadcrumbItem{
		{Name: "Catalogues", Link: "/contractAdmin"},
		{Name: catalog.Name, Link: ""},
	}
	return data, true
}

// ---- /distribution/listByDate/:date/:groupId ----

type EmargementConfigData struct {
	PageData
	DateISO  string
	DayLabel string
}

type EmargementMember struct {
	BasketNum   int
	UserName    string
	Coords      string
	Lines       []EmargementLine
	MemberTotal float64
}

type EmargementLine struct {
	Qty         string
	ProductName string
	CatalogName string
	UnitPrice   float64
	Fees        float64
	Total       float64
}

type EmargementPrintData struct {
	PageData
	GroupName  string
	DayLabel   string
	Place      string
	DateISO    string
	Members    []EmargementMember
	GrandTotal float64
	FontSize   string
	Mode       string
	Benevoles  []string
}

func (h *PagesHandler) DistributionListByDateConfigPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	dateStr := c.Param("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.String(http.StatusBadRequest, "date invalide")
		return
	}
	data := EmargementConfigData{
		PageData: pd,
		DateISO:  dateStr,
		DayLabel: frDayLabel(date),
	}
	data.Title = "Liste d'émargement — " + data.DayLabel

	t, err2 := loadTemplates("base.html", "design.html", "emargement_config.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

func (h *PagesHandler) DistributionListByDatePrintPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	dateStr := c.Param("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.String(http.StatusBadRequest, "date invalide")
		return
	}

	mode := c.DefaultQuery("mode", "all")
	fontSize := c.DefaultQuery("fontSize", "M")

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	var md model.MultiDistrib
	if err := h.db.Where("group_id = ? AND distrib_start_date >= ? AND distrib_start_date < ?",
		pd.Group.ID, dayStart, dayEnd).
		Preload("Place").
		Preload("Distributions.Catalog").
		First(&md).Error; err != nil {
		c.String(http.StatusNotFound, "aucune distribution ce jour")
		return
	}

	// Load volunteers
	var volunteers []string
	type volRow struct {
		FirstName string
		LastName  string
	}
	var vrows []volRow
	h.db.Raw(`SELECT u.first_name, u.last_name FROM volunteers v
		JOIN users u ON u.id = v.user_id
		WHERE v.multi_distrib_id = ?`, md.ID).Scan(&vrows)
	for _, v := range vrows {
		volunteers = append(volunteers, v.FirstName+" "+v.LastName)
	}

	// Collect orders
	type userEntry struct {
		name   string
		coords string
		lines  []EmargementLine
		total  float64
	}
	userMap := make(map[uint]*userEntry)
	userOrder := []uint{}

	for _, distrib := range md.Distributions {
		var orders []model.UserOrder
		h.db.Where("distribution_id = ?", distrib.ID).
			Preload("User").
			Preload("Product").
			Order("user_id").
			Find(&orders)

		for _, o := range orders {
			if _, exists := userMap[o.UserID]; !exists {
				coords := o.User.Email
				if o.User.Phone != nil && *o.User.Phone != "" {
					coords += " / " + *o.User.Phone
				}
				userMap[o.UserID] = &userEntry{
					name:   o.User.FirstName + " " + o.User.LastName,
					coords: coords,
				}
				userOrder = append(userOrder, o.UserID)
			}
			fees := o.TotalPrice() - o.Quantity*o.ProductPrice
			line := EmargementLine{
				Qty:         formatQty(o.Quantity, o.Product.UnitType),
				ProductName: o.Product.Name,
				CatalogName: distrib.Catalog.Name,
				UnitPrice:   o.ProductPrice,
				Fees:        fees,
				Total:       o.TotalPrice(),
			}
			userMap[o.UserID].lines = append(userMap[o.UserID].lines, line)
			userMap[o.UserID].total += o.TotalPrice()
		}
	}

	data := EmargementPrintData{
		PageData:   pd,
		GroupName:  pd.Group.Name,
		DayLabel:   frDayLabel(date),
		DateISO:    dateStr,
		FontSize:   fontSize,
		Mode:       mode,
		Benevoles:  volunteers,
	}
	if md.Place.ID != 0 {
		data.Place = md.Place.Name
	}
	data.Title = "Liste d'émargement — " + data.DayLabel

	for i, uid := range userOrder {
		u := userMap[uid]
		data.Members = append(data.Members, EmargementMember{
			BasketNum:   i + 1,
			UserName:    u.name,
			Coords:      u.coords,
			Lines:       u.lines,
			MemberTotal: u.total,
		})
		data.GrandTotal += u.total
	}

	t, err2 := loadTemplates("emargement_print.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err2 := t.ExecuteTemplate(c.Writer, "emargement_print.html", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- /contractAdmin/vendorsByDate/:date/:groupId ----

type VendorsByDateData struct {
	PageData
	DayLabel string
	DateISO  string
	Place    string
	Vendors  []VendorByDateEntry
}

type VendorByDateEntry struct {
	CatalogID   uint
	CatalogName string
	VendorName  string
	Lines       []VendorByDateLine
	Total       float64
}

type VendorByDateLine struct {
	Qty      string
	Ref      string
	Product  string
	UnitPrice float64
	Total    float64
}

func (h *PagesHandler) ContractAdminVendorsByDatePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	dateStr := c.Param("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.String(http.StatusBadRequest, "date invalide")
		return
	}

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	var md model.MultiDistrib
	if err := h.db.Where("group_id = ? AND distrib_start_date >= ? AND distrib_start_date < ?",
		pd.Group.ID, dayStart, dayEnd).
		Preload("Place").
		Preload("Distributions.Catalog.Vendor").
		First(&md).Error; err != nil {
		c.String(http.StatusNotFound, "aucune distribution ce jour")
		return
	}

	data := VendorsByDateData{
		PageData: pd,
		DayLabel: frDayLabel(date),
		DateISO:  dateStr,
	}
	if md.Place.ID != 0 {
		data.Place = md.Place.Name
	}
	data.Title = "Vue globale des commandes — " + data.DayLabel
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Distributions", Link: "/distribution"}}

	for _, distrib := range md.Distributions {
		var orders []model.UserOrder
		h.db.Where("distribution_id = ?", distrib.ID).
			Preload("Product").
			Order("product_id").
			Find(&orders)

		if len(orders) == 0 {
			continue
		}

		// Aggregate by product
		type productKey = uint
		type productAgg struct {
			ref       string
			name      string
			unitType  model.UnitType
			unitPrice float64
			qty       float64
			total     float64
		}
		aggMap := map[productKey]*productAgg{}
		aggOrder := []productKey{}

		for _, o := range orders {
			pid := o.ProductID
			if _, exists := aggMap[pid]; !exists {
				ref := ""
				if o.Product.Ref != nil {
					ref = *o.Product.Ref
				}
				aggMap[pid] = &productAgg{
					ref:      ref,
					name:     o.Product.Name,
					unitType: o.Product.UnitType,
					unitPrice: o.ProductPrice,
				}
				aggOrder = append(aggOrder, pid)
			}
			aggMap[pid].qty += o.Quantity
			aggMap[pid].total += o.TotalPrice()
		}

		entry := VendorByDateEntry{
			CatalogID:   distrib.CatalogID,
			CatalogName: distrib.Catalog.Name,
			VendorName:  distrib.Catalog.Vendor.Name,
		}
		for _, pid := range aggOrder {
			a := aggMap[pid]
			qty := strconv.FormatFloat(a.qty, 'f', -1, 64)
			entry.Lines = append(entry.Lines, VendorByDateLine{
				Qty:      qty,
				Ref:      a.ref,
				Product:  a.name,
				UnitPrice: a.unitPrice,
				Total:    a.total,
			})
			entry.Total += a.total
		}
		data.Vendors = append(data.Vendors, entry)
	}

	t, err2 := loadTemplates("base.html", "design.html", "contractadmin_vendors_by_date.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ensure time is used
var _ = time.Now

// ---- /contractAdmin/ordersByDate/:date/:groupId ----

type AvailableProduct struct {
	DistribID uint
	ProductID uint
	Name      string
	QtLabel   string
	Price     float64
	ImageURL  string
}

type AvailableCatalog struct {
	CatalogName string
	Products    []AvailableProduct
}

type OrdersByDateData struct {
	PageData
	Catalog           model.Catalog
	ActiveTab         string
	Date              string
	DayLabel          string
	StartHour         string
	Place             string
	MultiDistribID    uint
	DateISO           string
	BackURL           string
	Members           []OrdersByDateMember
	GrandTotal        float64
	GroupMembers      []MemberSelectItem
	AvailableCatalogs []AvailableCatalog
}

type MemberSelectItem struct {
	UserID   uint
	FullName string
}

type OrdersByDateMember struct {
	BasketNum  int
	UserID     uint
	UserName   string
	Lines      []OrdersByDateLine
	MemberTotal float64
}

type OrdersByDateLine struct {
	OrderID     uint
	CatalogName string
	CatalogID   uint
	DistribID   uint
	ProductID   uint
	Qty         string
	Quantity    float64
	Ref         string
	ProductName string
	UnitLabel   string
	UnitPrice   float64
	SubTotal    float64
	Fees        float64
	Total       float64
	Paid        bool
	ImageURL    string
}

var frDays = [7]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
var frMonths = [12]string{"Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}

func frDayLabel(t time.Time) string {
	return frDays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + " " + frMonths[t.Month()-1]
}

func (h *PagesHandler) ContractAdminOrdersByDatePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	dateStr := c.Param("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.String(http.StatusBadRequest, "date invalide")
		return
	}

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	var md model.MultiDistrib
	if err := h.db.Where("group_id = ? AND distrib_start_date >= ? AND distrib_start_date < ?",
		pd.Group.ID, dayStart, dayEnd).
		Preload("Place").
		Preload("Distributions.Catalog.Vendor").
		Preload("Distributions.Catalog.Products.Image").
		First(&md).Error; err != nil {
		c.String(http.StatusNotFound, "aucune distribution ce jour")
		return
	}

	// Collect all orders grouped by user
	type userEntry struct {
		name  string
		lines []OrdersByDateLine
		total float64
	}
	userMap := make(map[uint]*userEntry)
	userOrder := []uint{}

	for _, distrib := range md.Distributions {
		var orders []model.UserOrder
		h.db.Where("distribution_id = ?", distrib.ID).
			Preload("User").
			Preload("Product.Image").
			Order("user_id").
			Find(&orders)

		for _, o := range orders {
			if _, ok := userMap[o.UserID]; !ok {
				userMap[o.UserID] = &userEntry{
					name: o.User.FirstName + " " + o.User.LastName,
				}
				userOrder = append(userOrder, o.UserID)
			}
			fees := o.TotalPrice() - o.Quantity*o.ProductPrice
			ref := ""
			if o.Product.Ref != nil {
				ref = *o.Product.Ref
			}
			imgURL := ""
			if o.Product.Image != nil {
				imgURL = FileURL(o.Product.Image.ID, h.cfg.Key, o.Product.Image.Name)
			}
			line := OrdersByDateLine{
				OrderID:     o.ID,
				CatalogName: distrib.Catalog.Name,
				CatalogID:   distrib.CatalogID,
				DistribID:   distrib.ID,
				ProductID:   o.ProductID,
				Qty:         formatQty(o.Quantity, o.Product.UnitType),
				Quantity:    o.Quantity,
				Ref:         ref,
				ProductName: o.Product.Name,
				UnitLabel:   unitLabelFor(o.Product.UnitType),
				UnitPrice:   o.ProductPrice,
				SubTotal:    o.Quantity * o.ProductPrice,
				Fees:        fees,
				Total:       o.TotalPrice(),
				Paid:        o.Paid,
				ImageURL:    imgURL,
			}
			userMap[o.UserID].lines = append(userMap[o.UserID].lines, line)
			userMap[o.UserID].total += o.TotalPrice()
		}
	}

	data := OrdersByDateData{
		PageData:       pd,
		ActiveTab:      "orders",
		Date:           date.Format("02/01/2006"),
		DayLabel:       frDayLabel(date),
		StartHour:      md.DistribStartDate.Format("15:04"),
		DateISO:        dateStr,
		MultiDistribID: md.ID,
		BackURL:        c.Request.URL.RequestURI(),
	}
	if md.Place.ID != 0 {
		data.Place = md.Place.Name
	}
	if catalogIDStr := c.Query("catalog"); catalogIDStr != "" {
		if cid, err := strconv.ParseUint(catalogIDStr, 10, 64); err == nil {
			var cat model.Catalog
			if h.db.Preload("Vendor").First(&cat, cid).Error == nil {
				data.Catalog = cat
			}
		}
	}
	data.Title = "Distribution du " + data.DayLabel
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Distributions", Link: "/distribution"}}

	for i, uid := range userOrder {
		u := userMap[uid]
		data.Members = append(data.Members, OrdersByDateMember{
			BasketNum:   i + 1,
			UserID:      uid,
			UserName:    u.name,
			Lines:       u.lines,
			MemberTotal: u.total,
		})
		data.GrandTotal += u.total
	}

	for _, distrib := range md.Distributions {
		ac := AvailableCatalog{CatalogName: distrib.Catalog.Name}
		for _, p := range distrib.Catalog.Products {
			if !p.Active {
				continue
			}
			imgURL := ""
			if p.Image != nil {
				imgURL = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
			}
			qt := 1.0
			if p.Qt != nil {
				qt = *p.Qt
			}
			ac.Products = append(ac.Products, AvailableProduct{
				DistribID: distrib.ID,
				ProductID: p.ID,
				Name:      p.Name,
				QtLabel:   floatToFractionStr(qt) + " " + unitLabelFor(p.UnitType),
				Price:     p.Price,
				ImageURL:  imgURL,
			})
		}
		if len(ac.Products) > 0 {
			data.AvailableCatalogs = append(data.AvailableCatalogs, ac)
		}
	}

	var ugs []model.UserGroup
	h.db.Where("group_id = ?", pd.Group.ID).Preload("User").Find(&ugs)
	for _, ug := range ugs {
		data.GroupMembers = append(data.GroupMembers, MemberSelectItem{
			UserID:   ug.UserID,
			FullName: ug.User.LastName + " " + ug.User.FirstName,
		})
	}

	t, err2 := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_orders_by_date.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- GET /contractAdmin/ordersByDate/:date/:groupId/csv ----

func (h *PagesHandler) ContractAdminOrdersByDateCSV(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	dateStr := c.Param("date")
	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		c.String(http.StatusBadRequest, "date invalide")
		return
	}

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	dayEnd := dayStart.AddDate(0, 0, 1)

	var md model.MultiDistrib
	if err := h.db.Where("group_id = ? AND distrib_start_date >= ? AND distrib_start_date < ?",
		pd.Group.ID, dayStart, dayEnd).
		Preload("Distributions.Catalog.Vendor").
		First(&md).Error; err != nil {
		c.String(http.StatusNotFound, "aucune distribution ce jour")
		return
	}

	filename := "commandes-" + dateStr + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	w := c.Writer
	w.WriteString("\xEF\xBB\xBF") // BOM UTF-8 pour Excel
	w.WriteString("Membre;Catalogue;Qté;Réf.;Produit;P.U.;Sous-total;Frais;Total\n")

	for _, distrib := range md.Distributions {
		var orders []model.UserOrder
		h.db.Where("distribution_id = ?", distrib.ID).
			Preload("User").
			Preload("Product").
			Order("user_id").
			Find(&orders)

		for _, o := range orders {
			ref := ""
			if o.Product.Ref != nil {
				ref = *o.Product.Ref
			}
			fees := o.TotalPrice() - o.Quantity*o.ProductPrice
			memberName := o.User.FirstName + " " + o.User.LastName
			line := fmt.Sprintf("%s;%s;%s;%s;%s;%.2f;%.2f;%.2f;%.2f\n",
				memberName,
				distrib.Catalog.Name,
				formatQty(o.Quantity, o.Product.UnitType),
				ref,
				o.Product.Name,
				o.ProductPrice,
				o.Quantity*o.ProductPrice,
				fees,
				o.TotalPrice(),
			)
			w.WriteString(line)
		}
	}
}

// ---- GET/POST /contractAdmin/duplicate/:id ----

func (h *PagesHandler) CatalogAdminDuplicatePage(c *gin.Context) {
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

	var catalog model.Catalog
	if err := h.db.Preload("Products").Preload("Products.Image").First(&catalog, id).Error; err != nil {
		c.String(http.StatusNotFound, "catalogue introuvable")
		return
	}
	if catalog.GroupID != pd.Group.ID {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	if c.Request.Method == http.MethodPost {
		newName := strings.TrimSpace(c.PostForm("name"))
		if newName == "" {
			newName = catalog.Name + " - copie"
		}
		copyProducts := c.PostForm("copy_products") == "1"
		copyDistribs := c.PostForm("copy_distribs") == "1"

		// Create new catalog
		newCatalog := model.Catalog{
			GroupID:   catalog.GroupID,
			VendorID:  catalog.VendorID,
			ContactID: catalog.ContactID,
			Name:      newName,
			Flags:     catalog.Flags,
		}
		if err := h.db.Create(&newCatalog).Error; err != nil {
			c.String(http.StatusInternalServerError, "erreur création catalogue")
			return
		}

		// Copy products
		if copyProducts {
			for _, p := range catalog.Products {
				np := model.Product{
					CatalogID: newCatalog.ID,
					Name:      p.Name,
					Price:     p.Price,
					VAT:       p.VAT,
					UnitType:  p.UnitType,
					Organic:   p.Organic,
					Active:    p.Active,
				}
				if p.Ref != nil         { s := *p.Ref;         np.Ref = &s }
				if p.Description != nil { s := *p.Description; np.Description = &s }
				if p.Qt != nil          { f := *p.Qt;          np.Qt = &f }
				h.db.Create(&np)
			}
		}

		// Copy distributions (future only)
		if copyDistribs {
			var distribs []model.Distribution
			h.db.Where("catalog_id = ?", catalog.ID).Find(&distribs)
			now := time.Now()
			for _, d := range distribs {
				var md model.MultiDistrib
				if h.db.First(&md, d.MultiDistribID).Error != nil { continue }
				if md.DistribStartDate.Before(now) { continue }
				nd := model.Distribution{
					CatalogID:      newCatalog.ID,
					MultiDistribID: d.MultiDistribID,
				}
				if d.OrderStartDate != nil { t2 := *d.OrderStartDate; nd.OrderStartDate = &t2 }
				if d.OrderEndDate != nil   { t2 := *d.OrderEndDate;   nd.OrderEndDate = &t2 }
				h.db.Create(&nd)
			}
		}

		c.Redirect(http.StatusFound, "/contractAdmin")
		return
	}

	type DuplicatePage struct {
		PageData
		Catalog model.Catalog
	}
	dp := DuplicatePage{PageData: pd, Catalog: catalog}
	dp.Title = "Dupliquer — " + catalog.Name

	t, err := loadTemplates("base.html", "design.html", "contractadmin_duplicate.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", dp); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET+POST /contractAdmin/products/:id/importcsv ----

type ImportCSVData struct {
	CatalogAdminData
	Errors  []string
	Imported int
}

func (h *PagesHandler) CatalogAdminProductsImportCSV(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "products")
	if !ok {
		return
	}

	d := ImportCSVData{CatalogAdminData: data}
	d.Title = "Import CSV — " + data.Catalog.Name

	if c.Request.Method == http.MethodPost {
		file, err := c.FormFile("csv")
		if err != nil {
			d.Errors = append(d.Errors, "Fichier manquant.")
			renderImportCSV(c, d)
			return
		}
		f, err := file.Open()
		if err != nil {
			d.Errors = append(d.Errors, "Impossible d'ouvrir le fichier.")
			renderImportCSV(c, d)
			return
		}
		defer f.Close()

		r := csv.NewReader(f)
		r.Comma = ';'
		r.TrimLeadingSpace = true

		records, err := r.ReadAll()
		if err != nil {
			d.Errors = append(d.Errors, "Erreur de lecture CSV : "+err.Error())
			renderImportCSV(c, d)
			return
		}

		// Ignorer la ligne d'en-tête si présente
		start := 0
		if len(records) > 0 {
			if strings.ToLower(strings.TrimSpace(records[0][0])) == "nom" {
				start = 1
			}
		}

		unitMap := map[string]model.UnitType{
			"piece": model.UnitTypePiece, "pièce": model.UnitTypePiece, "Piece": model.UnitTypePiece,
			"kg": model.UnitTypeKilogram, "kilogram": model.UnitTypeKilogram,
			"g": model.UnitTypeGram, "gram": model.UnitTypeGram,
			"l": model.UnitTypeLitre, "litre": model.UnitTypeLitre,
			"cl": model.UnitTypeCentilitre, "centilitre": model.UnitTypeCentilitre,
			"ml": model.UnitTypeMillilitre, "millilitre": model.UnitTypeMillilitre,
		}

		for i, row := range records[start:] {
			line := start + i + 1
			if len(row) < 2 {
				d.Errors = append(d.Errors, fmt.Sprintf("Ligne %d ignorée (trop peu de colonnes)", line))
				continue
			}
			name := strings.TrimSpace(row[0])
			if name == "" {
				continue
			}
			price, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(row[1]), ",", "."), 64)
			if err != nil {
				d.Errors = append(d.Errors, fmt.Sprintf("Ligne %d : prix invalide (%s)", line, row[1]))
				continue
			}

			p := model.Product{
				Name:      name,
				Price:     price,
				CatalogID: data.Catalog.ID,
				Active:    true,
				UnitType:  model.UnitTypePiece,
			}

			col := func(idx int) string {
				if idx < len(row) {
					return strings.TrimSpace(row[idx])
				}
				return ""
			}

			if ref := col(2); ref != "" {
				p.Ref = &ref
			}
			if qt, err := parseFraction(col(3)); err == nil && qt != 0 {
				p.Qt = &qt
			}
			if u, ok := unitMap[strings.ToLower(col(4))]; ok {
				p.UnitType = u
			}
			if desc := col(5); desc != "" {
				p.Description = &desc
			}
			p.Organic = col(6) == "1" || strings.ToLower(col(6)) == "oui"
			if vat, err := strconv.ParseFloat(strings.ReplaceAll(col(7), ",", "."), 64); err == nil {
				p.VAT = vat
			}

			if err := h.db.Create(&p).Error; err != nil {
				d.Errors = append(d.Errors, fmt.Sprintf("Ligne %d : erreur DB : %v", line, err))
				continue
			}
			d.Imported++
		}

		if d.Imported > 0 && len(d.Errors) == 0 {
			c.Redirect(http.StatusFound, fmt.Sprintf("/contractAdmin/products/%d", data.Catalog.ID))
			return
		}
	}

	renderImportCSV(c, d)
}

// ---- /contractAdmin/selectDistrib/:id ----

type SelectDistribEntry struct {
	MultiDistribID uint
	DateLabel      string
	DateISO        string
	Place          string
	NbOrders       int
}

type SelectDistribData struct {
	CatalogAdminData
	Distribs []SelectDistribEntry
}

func (h *PagesHandler) CatalogAdminSelectDistribPage(c *gin.Context) {
	data, ok := h.loadCatalogAdmin(c, "orders")
	if !ok {
		return
	}

	var mds []model.MultiDistrib
	h.db.Joins("JOIN distributions ON distributions.multi_distrib_id = multi_distribs.id").
		Where("distributions.catalog_id = ? AND multi_distribs.group_id = ? AND multi_distribs.distrib_start_date >= ?",
			data.Catalog.ID, data.Group.ID, time.Now()).
		Preload("Place").
		Order("distrib_start_date ASC").
		Find(&mds)

	sd := SelectDistribData{CatalogAdminData: data}
	for _, md := range mds {
		var distrib model.Distribution
		h.db.Where("multi_distrib_id = ? AND catalog_id = ?", md.ID, data.Catalog.ID).First(&distrib)

		var nbOrders int64
		h.db.Model(&model.UserOrder{}).Where("distribution_id = ?", distrib.ID).Count(&nbOrders)

		place := ""
		if md.Place.ID != 0 {
			place = md.Place.Name
		}
		sd.Distribs = append(sd.Distribs, SelectDistribEntry{
			MultiDistribID: md.ID,
			DateLabel:      frDayLabel(md.DistribStartDate) + " à " + md.DistribStartDate.Format("15:04"),
			DateISO:        md.DistribStartDate.Format("2006-01-02"),
			Place:          place,
			NbOrders:       int(nbOrders),
		})
	}

	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_select_distrib.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", sd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func unitLabelFor(u model.UnitType) string {
	switch u {
	case model.UnitTypeKilogram:
		return "kg"
	case model.UnitTypeGram:
		return "g"
	case model.UnitTypeLitre:
		return "L"
	case model.UnitTypeCentilitre:
		return "cl"
	case model.UnitTypeMillilitre:
		return "ml"
	default:
		return "pièce"
	}
}

// ---- /contractAdmin/memberOrder/:multiDistribId/:userId ----

type MemberOrderProduct struct {
	DistribID   uint
	ProductID   uint
	Name        string
	Ref         string
	UnitLabel   string
	QtLabel     string
	Price       float64
	OrderID     uint
	Quantity    float64
	ImageURL    string
	HasOrder    bool
}

type MemberOrderCatalog struct {
	CatalogID   uint
	CatalogName string
	Products    []MemberOrderProduct
}

type MemberOrderData struct {
	PageData
	MemberName     string
	MemberID       uint
	MultiDistribID uint
	DateLabel      string
	BackURL        string
	Catalogs       []MemberOrderCatalog
	Saved          bool
}

func (h *PagesHandler) MemberOrderPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	mdIDStr := c.Param("multiDistribId")
	mdID, err := strconv.ParseUint(mdIDStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	userIDStr := c.Param("userId")
	targetUID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "userId invalide")
		return
	}

	var md model.MultiDistrib
	if err := h.db.Preload("Place").
		Preload("Distributions.Catalog.Vendor").
		Preload("Distributions.Catalog.Products.Image").
		First(&md, mdID).Error; err != nil {
		c.String(http.StatusNotFound, "distribution introuvable")
		return
	}

	var targetUser model.User
	if err := h.db.First(&targetUser, targetUID).Error; err != nil {
		c.String(http.StatusNotFound, "membre introuvable")
		return
	}

	if c.Request.Method == "POST" {
		c.Request.ParseForm()
		for _, distrib := range md.Distributions {
			for _, p := range distrib.Catalog.Products {
				key := fmt.Sprintf("qty_%d_%d", distrib.ID, p.ID)
				valStr := c.PostForm(key)
				qty, _ := strconv.ParseFloat(strings.TrimSpace(valStr), 64)

				var existing model.UserOrder
				found := h.db.Where("distribution_id = ? AND user_id = ? AND product_id = ?",
					distrib.ID, targetUID, p.ID).First(&existing).Error == nil

				if qty > 0 {
					feesRate := 0.0
					if distrib.Catalog.PercentageFees != nil {
						feesRate = *distrib.Catalog.PercentageFees
					}
					if found {
						h.db.Model(&existing).Updates(map[string]interface{}{
							"quantity":     qty,
							"product_price": p.Price,
							"fees_rate":    feesRate,
						})
					} else {
						h.db.Create(&model.UserOrder{
							UserID:         uint(targetUID),
							ProductID:      p.ID,
							DistributionID: &distrib.ID,
							Quantity:       qty,
							ProductPrice:   p.Price,
							FeesRate:       feesRate,
						})
					}
				} else if found {
					h.db.Delete(&existing)
				}
			}
		}
		back := c.Query("back")
		if back == "" {
			back = fmt.Sprintf("/contractAdmin/memberOrder/%d/%d", mdID, targetUID)
		}
		c.Redirect(http.StatusFound, back)
		return
	}

	// Load existing orders for this user on this multi-distrib
	orderMap := make(map[string]model.UserOrder)
	for _, distrib := range md.Distributions {
		var orders []model.UserOrder
		h.db.Where("distribution_id = ? AND user_id = ?", distrib.ID, targetUID).Find(&orders)
		for _, o := range orders {
			key := fmt.Sprintf("%d_%d", distrib.ID, o.ProductID)
			orderMap[key] = o
		}
	}

	data := MemberOrderData{
		PageData:       pd,
		MemberID:       uint(targetUID),
		MemberName:     targetUser.FirstName + " " + targetUser.LastName,
		MultiDistribID: uint(mdID),
		DateLabel:      frDayLabel(md.DistribStartDate) + " à " + md.DistribStartDate.Format("15:04"),
		BackURL:        c.Query("back"),
		Saved:          c.Query("saved") == "1",
	}
	data.Title = "Commande de " + data.MemberName
	data.Category = "distribution"

	for _, distrib := range md.Distributions {
		cat := MemberOrderCatalog{
			CatalogID:   distrib.CatalogID,
			CatalogName: distrib.Catalog.Name,
		}
		for _, p := range distrib.Catalog.Products {
			if !p.Active {
				continue
			}
			key := fmt.Sprintf("%d_%d", distrib.ID, p.ID)
			o := orderMap[key]
			ref := ""
			if p.Ref != nil {
				ref = *p.Ref
			}
			imgURL := ""
			if p.Image != nil {
				imgURL = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
			}
			qt := 1.0
			if p.Qt != nil {
				qt = *p.Qt
			}
			cat.Products = append(cat.Products, MemberOrderProduct{
				DistribID: distrib.ID,
				ProductID: p.ID,
				Name:      p.Name,
				Ref:       ref,
				UnitLabel: unitLabelFor(p.UnitType),
				QtLabel:   floatToFractionStr(qt) + " " + unitLabelFor(p.UnitType),
				Price:     p.Price,
				OrderID:   o.ID,
				Quantity:  o.Quantity,
				ImageURL:  imgURL,
				HasOrder:  o.ID != 0,
			})
		}
		if len(cat.Products) > 0 {
			data.Catalogs = append(data.Catalogs, cat)
		}
	}

	t, err2 := loadTemplates("base.html", "design.html", "contractadmin_member_order.html")
	if err2 != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err2)
		return
	}
	if err2 := t.ExecuteTemplate(c.Writer, "base", data); err2 != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err2)
	}
}

// ---- POST /contractAdmin/updateOrders/:multiDistribId/:userId ----

func (h *PagesHandler) UpdateMemberOrders(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "non autorisé"})
		return
	}
	mdIDStr := c.Param("multiDistribId")
	userIDStr := c.Param("userId")
	mdID, _ := strconv.ParseUint(mdIDStr, 10, 64)
	targetUID, _ := strconv.ParseUint(userIDStr, 10, 64)

	c.Request.ParseForm()
	for key, vals := range c.Request.PostForm {
		if !strings.HasPrefix(key, "qty_") {
			continue
		}
		orderIDStr := strings.TrimPrefix(key, "qty_")
		orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
		if err != nil {
			continue
		}
		qty, _ := strconv.ParseFloat(strings.TrimSpace(vals[0]), 64)
		paid := c.PostForm("paid_"+orderIDStr) == "1"

		var o model.UserOrder
		if h.db.First(&o, orderID).Error != nil {
			continue
		}
		if qty <= 0 {
			h.db.Delete(&o)
		} else {
			h.db.Model(&o).Updates(map[string]interface{}{
				"quantity": qty,
				"paid":     paid,
			})
		}
	}

	redirectURL := c.Query("back")
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("/contractAdmin/memberOrder/%d/%d", mdID, targetUID)
	}
	c.Redirect(http.StatusFound, redirectURL)
}

// ---- POST /contractAdmin/addProduct/:multiDistribId/:userId ----
// AJAX endpoint : ajoute (ou incrémente) une ligne de commande et retourne JSON.

type addProductReq struct {
	DistribID uint    `json:"distribId"`
	ProductID uint    `json:"productId"`
	Qty       float64 `json:"qty"`
}

type addProductResp struct {
	OrderID     uint    `json:"orderId"`
	ProductID   uint    `json:"productId"`
	DistribID   uint    `json:"distribId"`
	ProductName string  `json:"productName"`
	Ref         string  `json:"ref"`
	ImageURL    string  `json:"imageUrl"`
	UnitPrice   float64 `json:"unitPrice"`
	Quantity    float64 `json:"quantity"`
	QuantityStr string  `json:"quantityStr"`
	SubTotal    float64 `json:"subTotal"`
	Total       float64 `json:"total"`
	MemberTotal float64 `json:"memberTotal"`
	Created     bool    `json:"created"`
}

func (h *PagesHandler) AddMemberProduct(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "non autorisé"})
		return
	}

	mdID, _ := strconv.ParseUint(c.Param("multiDistribId"), 10, 64)
	userID, _ := strconv.ParseUint(c.Param("userId"), 10, 64)

	var req addProductReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Qty <= 0 {
		req.Qty = 1
	}

	var distrib model.Distribution
	if err := h.db.Preload("Catalog").
		Where("id = ? AND multi_distrib_id = ?", req.DistribID, mdID).
		First(&distrib).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution introuvable"})
		return
	}

	var prod model.Product
	if err := h.db.Preload("Image").
		Where("id = ? AND catalog_id = ?", req.ProductID, distrib.CatalogID).
		First(&prod).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "produit introuvable"})
		return
	}

	feesRate := 0.0
	if distrib.Catalog.PercentageFees != nil {
		feesRate = *distrib.Catalog.PercentageFees
	}

	var existing model.UserOrder
	created := false
	err := h.db.Where("distribution_id = ? AND user_id = ? AND product_id = ?",
		distrib.ID, userID, req.ProductID).First(&existing).Error
	if err != nil {
		existing = model.UserOrder{
			UserID:         uint(userID),
			ProductID:      req.ProductID,
			DistributionID: &distrib.ID,
			Quantity:       req.Qty,
			ProductPrice:   prod.Price,
			FeesRate:       feesRate,
		}
		if err := h.db.Create(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		created = true
	} else {
		existing.Quantity += req.Qty
		if err := h.db.Model(&existing).Update("quantity", existing.Quantity).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Member total sur toute la multi-distrib
	var allOrders []model.UserOrder
	h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ? AND user_orders.user_id = ?", mdID, userID).
		Find(&allOrders)
	memberTotal := 0.0
	for _, o := range allOrders {
		memberTotal += o.TotalPrice()
	}

	imageURL := ""
	if prod.Image != nil {
		imageURL = FileURL(prod.Image.ID, h.cfg.Key, prod.Image.Name)
	}
	ref := ""
	if prod.Ref != nil {
		ref = *prod.Ref
	}

	subTotal := existing.Quantity * prod.Price
	total := subTotal * (1 + feesRate/100)

	c.JSON(http.StatusOK, addProductResp{
		OrderID:     existing.ID,
		ProductID:   prod.ID,
		DistribID:   distrib.ID,
		ProductName: prod.Name,
		Ref:         ref,
		ImageURL:    imageURL,
		UnitPrice:   prod.Price,
		Quantity:    existing.Quantity,
		QuantityStr: strconv.FormatFloat(existing.Quantity, 'f', -1, 64),
		SubTotal:    subTotal,
		Total:       total,
		MemberTotal: memberTotal,
		Created:     created,
	})
}

// ---- POST /contractAdmin/deleteOrder/:multiDistribId/:userId/:orderId ----
// AJAX endpoint : supprime une ligne de commande, retourne JSON { memberTotal }.

func (h *PagesHandler) DeleteMemberOrder(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "non autorisé"})
		return
	}

	mdID, _ := strconv.ParseUint(c.Param("multiDistribId"), 10, 64)
	userID, _ := strconv.ParseUint(c.Param("userId"), 10, 64)
	orderID, _ := strconv.ParseUint(c.Param("orderId"), 10, 64)

	var o model.UserOrder
	if err := h.db.First(&o, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "commande introuvable"})
		return
	}
	if o.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "commande d'un autre membre"})
		return
	}
	if err := h.db.Delete(&o).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var allOrders []model.UserOrder
	h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ? AND user_orders.user_id = ?", mdID, userID).
		Find(&allOrders)
	memberTotal := 0.0
	for _, x := range allOrders {
		memberTotal += x.TotalPrice()
	}

	c.JSON(http.StatusOK, gin.H{"memberTotal": memberTotal})
}

func renderImportCSV(c *gin.Context, data ImportCSVData) {
	t, err := loadTemplates("base.html", "design.html", "contractadmin_layout.html", "contractadmin_products_importcsv.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
