// Package handler — endpoints compatibles avec l'API originale Alterconso (app.js).
// Ces routes reproduisent exactement les URL et formats JSON attendus par le frontend Haxe compilé.
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type CompatHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewCompatHandler(db *gorm.DB, cfg *config.Config) *CompatHandler {
	return &CompatHandler{db: db, cfg: cfg}
}

// ---- /api/user/login ----
// POST avec form-data : email, password
// Réponse : {"success":true,"token":"JWT"} ou {"error":{"message":"..."}}

func (h *CompatHandler) UserLogin(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	var user model.User
	if err := h.db.Where("email = ? OR email2 = ?", email, email).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Email ou mot de passe incorrect."}})
		return
	}
	if !user.CheckPassword(password, h.cfg.Key) {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Email ou mot de passe incorrect."}})
		return
	}
	if user.EmailVerifiedAt == nil {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Votre compte n'est pas encore activé. Vérifiez votre boîte mail."}})
		return
	}

	now := time.Now()
	h.db.Model(&user).Update("last_login", now)

	claims := &middleware.Claims{
		UserID:  user.ID,
		GroupID: 0,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * 7 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Erreur interne."}})
		return
	}

	// Cookie httpOnly pour les appels API suivants
	c.SetCookie("token", signed, 3600*24*7, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true, "token": signed})
}

// ---- /api/user/me ----

func (h *CompatHandler) UserMe(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, userInfo(user))
}

// ---- /api/user/getFromGroup/ ----
// Retourne les membres du groupe courant.

func (h *CompatHandler) UserGetFromGroup(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims.GroupID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no group selected"})
		return
	}
	var ugs []model.UserGroup
	h.db.Where("group_id = ?", claims.GroupID).Preload("User").Find(&ugs)

	users := make([]gin.H, 0, len(ugs))
	for _, ug := range ugs {
		users = append(users, userInfo(ug.User))
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// ---- /api/order/catalogs/:multiDistribId ----
// Retourne les catalogues d'un MultiDistrib.

func (h *CompatHandler) OrderCatalogs(c *gin.Context) {
	mdID, err := strconv.ParseUint(c.Param("multiDistribId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var md model.MultiDistrib
	if err := h.db.Preload("Distributions.Catalog.Vendor").First(&md, mdID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	catalogs := make([]gin.H, 0)
	for _, d := range md.Distributions {
		cat := d.Catalog
		catalogs = append(catalogs, gin.H{
			"id":    cat.ID,
			"name":  cat.Name,
			"image": nil,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "catalogs": catalogs})
}

// ---- /api/order/get/:userId ----
// ?catalog=<catalogId>&multiDistrib=<multiDistribId>

func (h *CompatHandler) OrderGet(c *gin.Context) {
	claims := middleware.GetClaims(c)
	userIDParam, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
		return
	}
	// Only allow own orders or group manager
	if uint(userIDParam) != claims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	catalogID, _ := strconv.ParseUint(c.Query("catalog"), 10, 64)
	mdID, _ := strconv.ParseUint(c.Query("multiDistrib"), 10, 64)

	query := h.db.Where("user_orders.user_id = ?", userIDParam).
		Preload("Product").
		Preload("Product.Catalog")

	if mdID != 0 {
		query = query.
			Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
			Where("distributions.multi_distrib_id = ?", mdID)
	} else if catalogID != 0 {
		query = query.
			Joins("JOIN products ON products.id = user_orders.product_id").
			Where("products.catalog_id = ?", catalogID)
	}

	var orders []model.UserOrder
	query.Find(&orders)

	out := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderInfo(o))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "orders": out})
}

// ---- /api/order/update/:userId ----
// Body JSON : {"orders":[{"productId":1,"qt":2,...}]}

func (h *CompatHandler) OrderUpdate(c *gin.Context) {
	claims := middleware.GetClaims(c)
	userIDParam, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || uint(userIDParam) != claims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	catalogID, _ := strconv.ParseUint(c.Query("catalog"), 10, 64)
	mdID, _ := strconv.ParseUint(c.Query("multiDistrib"), 10, 64)

	var body struct {
		Orders []struct {
			ID        *uint   `json:"id"`
			ProductID uint    `json:"productId"`
			Qt        float64 `json:"qt"`
			Paid      bool    `json:"paid"`
		} `json:"orders"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the distribution for this multiDistrib + catalog
	var distribID *uint
	if mdID != 0 && catalogID != 0 {
		var d model.Distribution
		if err := h.db.Where("multi_distrib_id = ? AND catalog_id = ?", mdID, catalogID).
			First(&d).Error; err == nil {
			distribID = &d.ID
		}
	}

	out := make([]gin.H, 0, len(body.Orders))
	for _, item := range body.Orders {
		if item.Qt == 0 {
			// Delete order
			if item.ID != nil {
				h.db.Delete(&model.UserOrder{}, *item.ID)
			}
			continue
		}

		// Get product price
		var product model.Product
		if err := h.db.Preload("Catalog").First(&product, item.ProductID).Error; err != nil {
			continue
		}
		feesRate := 0.0
		if product.Catalog.PercentageFees != nil {
			feesRate = *product.Catalog.PercentageFees
		}

		if item.ID != nil {
			// Update existing order
			h.db.Model(&model.UserOrder{}).Where("id = ?", *item.ID).
				Updates(map[string]interface{}{"quantity": item.Qt, "paid": item.Paid})
			var o model.UserOrder
			h.db.Preload("Product").Preload("Product.Catalog").First(&o, *item.ID)
			out = append(out, orderInfo(o))
		} else {
			// Create new order
			o := model.UserOrder{
				UserID:         uint(userIDParam),
				ProductID:      item.ProductID,
				Quantity:       item.Qt,
				ProductPrice:   product.Price,
				FeesRate:       feesRate,
				Paid:           item.Paid,
				DistributionID: distribID,
			}
			h.db.Create(&o)
			h.db.Preload("Product").Preload("Product.Catalog").First(&o, o.ID)
			out = append(out, orderInfo(o))
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "orders": out})
}

// ---- /api/product/get/ ----
// ?catalogId=<id>

func (h *CompatHandler) ProductGet(c *gin.Context) {
	catalogID, err := strconv.ParseUint(c.Query("catalogId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing catalogId"})
		return
	}

	var products []model.Product
	h.db.Where("catalog_id = ?", catalogID).Preload("Catalog").Find(&products)

	out := make([]gin.H, 0, len(products))
	for _, p := range products {
		out = append(out, productInfo(p))
	}
	c.JSON(http.StatusOK, gin.H{"products": out})
}

// ---- /api/planning/:groupId ----

func (h *CompatHandler) Planning(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("groupId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid groupId"})
		return
	}

	var distribs []model.MultiDistrib
	h.db.Where("group_id = ? AND distrib_end_date >= ?", groupID, time.Now()).
		Preload("Place").
		Preload("Distributions.Catalog").
		Order("distrib_start_date ASC").
		Limit(50).
		Find(&distribs)

	out := make([]gin.H, 0)
	for _, md := range distribs {
		for _, d := range md.Distributions {
			out = append(out, gin.H{
				"id":         d.ID,
				"start":      md.DistribStartDate,
				"end":        md.DistribEndDate,
				"contract":   d.Catalog.Name,
				"contractId": d.CatalogID,
				"place":      md.Place.Name,
			})
		}
	}
	c.JSON(http.StatusOK, out)
}

// ---- Serializers ----

func userInfo(u model.User) gin.H {
	out := gin.H{
		"id":        u.ID,
		"name":      u.FirstName + " " + u.LastName,
		"firstName": u.FirstName,
		"lastName":  u.LastName,
		"email":     u.Email,
	}
	if u.Phone != nil {
		out["phone"] = *u.Phone
	}
	if u.City != nil {
		out["city"] = *u.City
	}
	if u.ZipCode != nil {
		out["zipCode"] = *u.ZipCode
	}
	if u.Address1 != nil {
		out["address1"] = *u.Address1
	}
	return out
}

// normalizeUnitType ensures the value matches the Haxe enum constructor names.
// Legacy DB values may store "Unit" instead of "Piece".
func normalizeUnitType(u model.UnitType) string {
	if u == "Unit" {
		return "Piece"
	}
	return string(u)
}

// unitTypeIndex returns the numeric index expected by Type.createEnumIndex in the Haxe frontend.
// Order: ["Piece","Kilogram","Gram","Litre","Centilitre","Millilitre"]
func unitTypeIndex(u model.UnitType) int {
	switch u {
	case model.UnitTypeKilogram:
		return 1
	case model.UnitTypeGram:
		return 2
	case model.UnitTypeLitre:
		return 3
	case model.UnitTypeCentilitre:
		return 4
	case model.UnitTypeMillilitre:
		return 5
	default: // "Piece", "Unit", or anything else
		return 0
	}
}

// shopProductInfo is like productInfo but returns unitType as a numeric index
// as expected by the React shop component (Type.createEnumIndex).
func shopProductInfo(p model.Product) gin.H {
	h := productInfo(p)
	h["unitType"] = unitTypeIndex(p.UnitType)
	return h
}

func productInfo(p model.Product) gin.H {
	taxRate := 0.0
	taxName := ""
	if p.Catalog.PercentageFees != nil {
		taxRate = *p.Catalog.PercentageFees
		if p.Catalog.PercentageName != nil {
			taxName = *p.Catalog.PercentageName
		}
	}
	return gin.H{
		"id":            p.ID,
		"name":          p.Name,
		"ref":           "",
		"image":         nil,
		"price":         p.Price,
		"vat":           p.VAT,
		"vatValue":      p.Price * p.VAT / 100,
		"desc":          "",
		"categories":    []int{},
		"subcategories": []int{},
		"orderable":     true,
		"stock":         p.Stock,
		"hasFloatQt":    false,
		"qt":            0,
		"unitType":      normalizeUnitType(p.UnitType),
		"organic":       p.Organic,
		"variablePrice": false,
		"wholesale":     false,
		"active":        true,
		"bulk":          false,
		"catalogId":     p.CatalogID,
		"catalogTax":    taxRate,
		"catalogTaxName": taxName,
	}
}

func orderInfo(o model.UserOrder) gin.H {
	smartQt := fmt.Sprintf("%.0f", o.Quantity)
	total := o.TotalPrice()
	return gin.H{
		"id":                 o.ID,
		"userId":             o.UserID,
		"userName":           o.User.FirstName + " " + o.User.LastName,
		"product":            productInfo(o.Product),
		"quantity":           o.Quantity,
		"smartQt":            smartQt,
		"subTotal":           o.Quantity * o.ProductPrice,
		"total":              total,
		"paid":               o.Paid,
		"invertSharedOrder":  false,
		"catalogId":          o.Product.CatalogID,
		"catalogName":        o.Product.Catalog.Name,
	}
}

// ---- /api/user/register ----
// POST form-data : email, password, firstName, lastName

func (h *CompatHandler) UserRegister(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	firstName := strings.TrimSpace(c.PostForm("firstName"))
	lastName := strings.TrimSpace(c.PostForm("lastName"))

	if email == "" || password == "" || firstName == "" || lastName == "" {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Tous les champs sont requis."}})
		return
	}

	// Check email uniqueness
	var existing model.User
	if err := h.db.Where("email = ?", email).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Cet email est déjà utilisé."}})
		return
	}

	user := model.User{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}
	user.SetPassword(password, h.cfg.Key)

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Erreur lors de la création du compte."}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "user": userInfo(user)})
}

// ---- /api/shop/init/:multiDistribId ----

func (h *CompatHandler) ShopInit(c *gin.Context) {
	mdIDStr := c.Query("multiDistrib")
	if mdIDStr == "" {
		mdIDStr = c.Param("multiDistribId")
	}
	mdID, err := strconv.ParseUint(mdIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var md model.MultiDistrib
	if err := h.db.Preload("Place").
		Preload("Distributions.Catalog.Vendor").
		First(&md, mdID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	catalogs := make([]gin.H, 0)
	for _, d := range md.Distributions {
		cat := d.Catalog
		catalogs = append(catalogs, gin.H{
			"id":        cat.ID,
			"name":      cat.Name,
			"vendorId":  cat.VendorID,
			"vendor":    gin.H{"id": cat.Vendor.ID, "name": cat.Vendor.Name},
			"canOrder":  d.CanOrderNow(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"multiDistrib": gin.H{
			"id":    md.ID,
			"start": md.DistribStartDate,
			"end":   md.DistribEndDate,
			"place": md.Place.Name,
		},
		"catalogs": catalogs,
	})
}

// ---- /api/shop/allProducts/:multiDistribId ----

func (h *CompatHandler) ShopAllProducts(c *gin.Context) {
	mdIDStr := c.Query("multiDistrib")
	if mdIDStr == "" {
		mdIDStr = c.Param("multiDistribId")
	}
	mdID, err := strconv.ParseUint(mdIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var distribs []model.Distribution
	h.db.Where("multi_distrib_id = ?", mdID).
		Preload("Catalog").
		Find(&distribs)

	catalogIDs := make([]uint, 0, len(distribs))
	for _, d := range distribs {
		catalogIDs = append(catalogIDs, d.CatalogID)
	}

	var products []model.Product
	if len(catalogIDs) > 0 {
		h.db.Where("catalog_id IN ?", catalogIDs).Preload("Catalog").Find(&products)
	}

	out := make([]gin.H, 0, len(products))
	for _, p := range products {
		out = append(out, shopProductInfo(p))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "products": out})
}

// ---- /api/shop/categories ----

func (h *CompatHandler) ShopCategories(c *gin.Context) {
	// Categories are not implemented in this version; return empty list
	c.JSON(http.StatusOK, gin.H{"success": true, "categories": []gin.H{}})
}

// ---- /api/product/categories ----

func (h *CompatHandler) ProductCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "categories": []gin.H{}})
}

// ---- POST /api/shop/submit/:multiDistribId ----
// Body JSON: {"catalogId":1,"orders":[{"productId":1,"qt":2}]}

func (h *CompatHandler) ShopSubmit(c *gin.Context) {
	claims := middleware.GetClaims(c)
	mdID, err := strconv.ParseUint(c.Param("multiDistribId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		CatalogID uint `json:"catalogId"`
		Orders    []struct {
			ProductID uint    `json:"productId"`
			Qt        float64 `json:"qt"`
		} `json:"orders"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the distribution for this multiDistrib + catalog
	var distrib model.Distribution
	if err := h.db.Where("multi_distrib_id = ? AND catalog_id = ?", mdID, body.CatalogID).
		First(&distrib).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution not found"})
		return
	}

	// Delete existing orders for this user + distribution
	h.db.Where("user_id = ? AND distribution_id = ?", claims.UserID, distrib.ID).
		Delete(&model.UserOrder{})

	out := make([]gin.H, 0)
	for _, item := range body.Orders {
		if item.Qt <= 0 {
			continue
		}
		var product model.Product
		if err := h.db.Preload("Catalog").First(&product, item.ProductID).Error; err != nil {
			continue
		}
		feesRate := 0.0
		if product.Catalog.PercentageFees != nil {
			feesRate = *product.Catalog.PercentageFees
		}
		o := model.UserOrder{
			UserID:         claims.UserID,
			ProductID:      item.ProductID,
			Quantity:       item.Qt,
			ProductPrice:   product.Price,
			FeesRate:       feesRate,
			DistributionID: &distrib.ID,
		}
		h.db.Create(&o)
		h.db.Preload("Product").Preload("Product.Catalog").First(&o, o.ID)
		out = append(out, orderInfo(o))
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "orders": out})
}

// smartQty formats a quantity with its unit label.
func smartQty(qty float64, unit model.UnitType) string {
	switch unit {
	case model.UnitTypeKilogram:
		if qty < 1 {
			return fmt.Sprintf("%.0fg", qty*1000)
		}
		return fmt.Sprintf("%.2fkg", qty)
	case model.UnitTypeGram:
		return fmt.Sprintf("%.0fg", qty)
	case model.UnitTypeLitre:
		return fmt.Sprintf("%.2fL", qty)
	default:
		if qty == float64(int(qty)) {
			return strconv.Itoa(int(qty))
		}
		return fmt.Sprintf("%.2f", qty)
	}
}
