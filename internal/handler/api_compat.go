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
	ok, needsRehash := user.CheckPassword(password, h.cfg.Key)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Email ou mot de passe incorrect."}})
		return
	}
	if user.EmailVerifiedAt == nil {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"message": "Votre compte n'est pas encore activé. Vérifiez votre boîte mail."}})
		return
	}

	// Migration opportuniste : on détient le clair, on réécrit vers bcrypt (b2:).
	if needsRehash {
		user.SetPassword(password, "")
		h.db.Model(&user).Update("pass", user.Pass)
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
	out := userInfo(user)
	out["isAdmin"] = isTechnicalManagerEmail(user.Email)
	// hasDatabaseAdmin : droit gating de l'accès à /admin/db. Calé sur la même
	// logique que PageData.HasDatabaseAdmin (cf. pages.go::buildPageData).
	if claims.GroupID != 0 {
		ug := loadGroupAccess(h.db, claims.UserID, claims.GroupID)
		if ug != nil {
			out["hasDatabaseAdmin"] = ug.CanAdminDatabase()
		}
	}
	c.JSON(http.StatusOK, out)
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

// callerManages dit si l'appelant administre ce groupe. Sert à lever les
// bornes de clôture pour qui vient corriger une commande.
func (h *CompatHandler) callerManages(c *gin.Context, groupID uint) bool {
	claims := middleware.GetClaims(c)
	if claims == nil || groupID == 0 {
		return false
	}
	ug := loadGroupAccess(h.db, claims.UserID, groupID)
	return ug != nil && ug.IsGroupManager()
}

// groupOfOrderScope retourne le groupe auquel se rattache une consultation de
// commandes, désigné soit par la distribution, soit par le catalogue. Zéro
// quand aucun des deux ne permet de conclure — l'appelant refuse alors l'accès
// plutôt que de deviner.
func (h *CompatHandler) groupOfOrderScope(mdID, catalogID uint64) uint {
	if mdID != 0 {
		var md model.MultiDistrib
		if h.db.Select("id, group_id").First(&md, mdID).Error == nil {
			return md.GroupID
		}
	}
	if catalogID != 0 {
		var cat model.Catalog
		if h.db.Select("id, group_id").First(&cat, catalogID).Error == nil {
			return cat.GroupID
		}
	}
	return 0
}

func (h *CompatHandler) OrderGet(c *gin.Context) {
	claims := middleware.GetClaims(c)
	userIDParam, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
		return
	}
	catalogID, _ := strconv.ParseUint(c.Query("catalog"), 10, 64)
	mdID, _ := strconv.ParseUint(c.Query("multiDistrib"), 10, 64)

	// Ses propres commandes, ou celles d'un membre quand on gère son groupe —
	// ce que le commentaire d'origine annonçait sans que le code le fasse.
	//
	// C'est ce que consulte le shop pour pré-remplir le panier avant de
	// commander pour autrui. Le refuser ne protégeait rien : le panier partait
	// vide, et la validation écrasait la commande du membre au lieu de la
	// modifier.
	if uint(userIDParam) != claims.UserID {
		groupID := h.groupOfOrderScope(mdID, catalogID)
		ug := loadGroupAccess(h.db, claims.UserID, groupID)
		if groupID == 0 || ug == nil || !ug.IsGroupManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

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
	ref := ""
	if p.Ref != nil {
		ref = *p.Ref
	}
	desc := ""
	if p.Description != nil {
		desc = *p.Description
	}
	// Un produit sans quantité renseignée en vaut une : c'est la convention que
	// suivent déjà les pages d'administration. Renvoyer zéro laissait le shop
	// sans quantité ni prix unitaire à afficher, les deux se calculant à partir
	// de cette valeur.
	qt := 1.0
	if p.Qt != nil && *p.Qt != 0 {
		qt = *p.Qt
	}
	return gin.H{
		"id":            p.ID,
		"name":          p.Name,
		"ref":           ref,
		"image":         nil,
		"price":         p.Price,
		"vat":           p.VAT,
		"vatValue":      p.Price * p.VAT / 100,
		"desc":          desc,
		"categories":    []int{},
		"subcategories": []int{},
		"orderable":     true,
		"stock":         p.Stock,
		"hasFloatQt":    p.HasFloatQt,
		"qt":            qt,
		"unitType":      normalizeUnitType(p.UnitType),
		"organic":       p.Organic,
		"variablePrice": p.VariablePrice,
		"wholesale":     p.MultiWeight,
		"active":        p.Active,
		"bulk":          false,
		"catalogId":     p.CatalogID,
		"catalogTax":    taxRate,
		"catalogTaxName": taxName,
		"vendorId":      p.Catalog.VendorID,
		"resaleFrom":    p.ResaleFrom,
	}
}

// vendorInfo construit la VendorInfos JSON attendue par le shop legacy.
// Beaucoup de champs (profession, linkText, longDesc, …) n'existent pas
// dans le modèle Go ; on retourne une chaîne vide pour préserver la forme.
//
// `image` est sérialisé null (pas "") quand le vendor n'a pas d'image, sinon
// le composant Avatar de Material-UI affiche son fallback (icône personnage).
func vendorInfo(v model.Vendor) gin.H {
	desc := ""
	if v.Description != nil {
		desc = *v.Description
	}
	zip := ""
	if v.ZipCode != nil {
		zip = *v.ZipCode
	}
	city := ""
	if v.City != nil {
		city = *v.City
	}
	var image interface{} // nil → JSON null
	if v.ImagePath != nil && *v.ImagePath != "" {
		image = *v.ImagePath
	}
	return gin.H{
		"id":         v.ID,
		"name":       v.Name,
		"desc":       desc,
		"longDesc":   desc,
		"image":      image,
		"profession": "",
		"zipCode":    zip,
		"city":       city,
		"linkText":   "",
		"linkUrl":    "",
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
		"productPrice":       o.ProductPrice,
		"subTotal":           o.Quantity * o.ProductPrice,
		"feesRate":           o.FeesRate,
		"total":              total,
		"paid":               o.Paid,
		"invertSharedOrder":  false,
		"catalogId":          o.Product.CatalogID,
		"catalogName":        o.Product.Catalog.Name,
	}
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
		Preload("Group").
		Preload("Distributions.Catalog.Vendor").
		First(&md, mdID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	catalogs := make([]gin.H, 0)
	vendors := make([]gin.H, 0)
	seenVendor := make(map[uint]bool)
	// Même dispense que pour le rayon, et même limite : après la clôture oui,
	// avant l'ouverture non.
	manager := h.callerManages(c, md.GroupID)
	now := time.Now()

	for _, d := range md.Distributions {
		cat := d.Catalog
		// Preload("Distributions...") ne remplit pas le lien retour vers le
		// jour : sans lui, CanOrderNow ne voyait pas la clôture commune et
		// déclarait ouverts tous les catalogues qui n'en surchargeaient pas.
		d.MultiDistrib = md
		catalogs = append(catalogs, gin.H{
			"id":       cat.ID,
			"name":     cat.Name,
			"vendorId": cat.VendorID,
			"vendor":   gin.H{"id": cat.Vendor.ID, "name": cat.Vendor.Name},
			"canOrder": d.CanOrderNow() || (manager && d.OrderWindowStarted(now)),
		})
		if !seenVendor[cat.VendorID] {
			vendors = append(vendors, vendorInfo(cat.Vendor))
			seenVendor[cat.VendorID] = true
		}
	}

	// Champs au top-level attendus par CagetteStore.componentDidMount :
	// place, distributionStartDate (parseable par Date.fromString), orderEndDates,
	// vendors, paymentInfos. Si distributionStartDate est undefined, le Haxe
	// throw → setState({vendors}) ne s'exécute jamais et la lookup vendor null.
	c.JSON(http.StatusOK, gin.H{
		"success":               true,
		"place":                 placeInfos(md.Place),
		"group":                 gin.H{"id": md.Group.ID, "name": md.Group.Name},
		"distributionStartDate": md.DistribStartDate.Format("2006-01-02 15:04:05"),
		"distributionEndDate":   md.DistribEndDate.Format("2006-01-02 15:04:05"),
		"orderEndDates":         []gin.H{},
		"vendors":               vendors,
		"paymentInfos":          "",
		// Conservés pour ne pas casser un éventuel consommateur tiers.
		"multiDistrib": gin.H{
			"id":    md.ID,
			"start": md.DistribStartDate,
			"end":   md.DistribEndDate,
			"place": md.Place.Name,
		},
		"catalogs": catalogs,
	})
}

// placeInfos retourne la PlaceInfos JSON attendue par le shop legacy.
func placeInfos(p model.Place) gin.H {
	addr := ""
	if p.Address != nil {
		addr = *p.Address
	}
	zip := ""
	if p.ZipCode != nil {
		zip = *p.ZipCode
	}
	city := ""
	if p.City != nil {
		city = *p.City
	}
	lat := 0.0
	if p.Lat != nil {
		lat = *p.Lat
	}
	lng := 0.0
	if p.Lng != nil {
		lng = *p.Lng
	}
	return gin.H{
		"id":        p.ID,
		"name":      p.Name,
		"address1":  addr,
		"address2":  "",
		"zipCode":   zip,
		"city":      city,
		"latitude":  lat,
		"longitude": lng,
	}
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

	// Le jour porte la clôture par défaut, mais c'est celle du catalogue qui
	// fait foi : un producteur qui a repoussé la sienne reste commandable
	// quand les autres sont clos, et l'inverse est vrai aussi.
	var md model.MultiDistrib
	if err := h.db.First(&md, mdID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var distribs []model.Distribution
	h.db.Where("multi_distrib_id = ?", mdID).
		Preload("Catalog").
		Find(&distribs)

	// Les catalogues CLOS restent en rayon, et c'est délibéré. Les en retirer
	// les faisait disparaître sans un mot : l'adhérent cherchait son
	// producteur habituel, ne le trouvait pas, et rien ne lui disait qu'il
	// avait simplement fermé. Le shop les affiche désormais estompés, avec
	// « Commandes closes » à la place du bouton — il lit la clôture par
	// catalogue dans le « canOrder » que /api/shop/init renvoie déjà.
	//
	// Un gestionnaire, lui, les garde commandables : /api/shop/init lui répond
	// canOrder=true sur un catalogue clos, car corriger une commande après la
	// clôture fait partie de son travail. Le rayon n'a donc plus à distinguer
	// qui regarde.
	//
	// Deux cas sortent quand même :
	//   - le catalogue qui n'accepte pas la commande en ligne, qui n'a jamais
	//     eu sa place ici ;
	//   - celui dont l'ouverture n'est pas venue, qu'annoncer d'avance ne
	//     ferait qu'égarer — et pour lequel « closes » serait un contresens.
	now := time.Now()

	catalogIDs := make([]uint, 0, len(distribs))
	for _, d := range distribs {
		d.MultiDistrib = md
		if !d.ShowsInShop(now) {
			continue
		}
		catalogIDs = append(catalogIDs, d.CatalogID)
	}

	var products []model.Product
	if len(catalogIDs) > 0 {
		h.db.Where("catalog_id IN ? AND active = ?", catalogIDs, true).
			Preload("Catalog").
			Preload("TxpSubCategory").
			Preload("Image").
			Order("name").
			Find(&products)
	}

	// Charge toutes les catégories pour :
	//   - identifier la catégorie de fallback "Autres / Tous"
	//   - construire les maps id → image utilisées pour le fallback d'image
	//     produit (l'icône de catégorie remplace l'absence de visuel produit)
	var allCats []model.TxpCategory
	h.db.Preload("SubCategories").Find(&allCats)
	catImageByID := make(map[uint]string, len(allCats))
	for _, c := range allCats {
		catImageByID[c.ID] = c.Image
	}
	var fallback model.TxpCategory
	for _, c := range allCats {
		if c.Image == "autres" {
			fallback = c
			break
		}
	}
	fallbackCatID := fallback.ID
	var fallbackSubID uint
	if len(fallback.SubCategories) > 0 {
		fallbackSubID = fallback.SubCategories[0].ID
	}

	out := make([]gin.H, 0, len(products))
	for _, p := range products {
		info := shopProductInfo(p)
		catID, subID := fallbackCatID, fallbackSubID
		if p.TxpSubCategory != nil {
			subID = p.TxpSubCategory.ID
			catID = p.TxpSubCategory.CategoryID
		}
		info["categories"] = []uint{catID}
		info["subcategories"] = []uint{subID}
		// Image : préférence à l'image du produit ; sinon illustration de la
		// catégorie (les fichiers sous /img/taxo/grey/ sont les illustrations
		// 300×300 — le dossier est mal nommé, ce ne sont pas des grises).
		if p.Image != nil {
			info["image"] = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
		} else if img, ok := catImageByID[catID]; ok && img != "" {
			info["image"] = "/img/taxo/grey/" + img + ".png"
		}
		out = append(out, info)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "products": out})
}

// ---- /api/shop/categories ----

func (h *CompatHandler) ShopCategories(c *gin.Context) {
	var cats []model.TxpCategory
	h.db.Preload("SubCategories").Order("display_order").Find(&cats)

	out := make([]gin.H, 0, len(cats))
	for _, cat := range cats {
		subs := make([]gin.H, 0, len(cat.SubCategories))
		for _, sub := range cat.SubCategories {
			subs = append(subs, gin.H{"id": sub.ID, "name": sub.Name})
		}
		out = append(out, gin.H{
			"id":            cat.ID,
			"name":          cat.Name,
			"image":         "/img/taxo/" + cat.Image + ".png",
			"displayOrder":  cat.DisplayOrder,
			"subcategories": subs,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "categories": out})
}

// ---- /api/product/categories ----

func (h *CompatHandler) ProductCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "categories": []gin.H{}})
}

// ---- GET /api/session ----

// SessionInfo décrit la session courante pour les écrans servis par la SPA.
//
// Les pages rendues par Go affichent le bandeau « connecté en tant que » depuis
// design.html, via PageData.IsImpersonating. Les routes SPA (/shop, /login,
// /profile, /groups) sont servies par frontend/dist/index.html sans passer par
// un template : aucun bandeau ne les entoure, et l'utilisateur perdait à la
// fois l'avertissement et le lien de retour. Cet endpoint donne au front les
// mêmes informations.
func (h *CompatHandler) SessionInfo(c *gin.Context) {
	out := gin.H{"impersonating": false}

	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusOK, out)
		return
	}

	var u model.User
	if h.db.First(&u, claims.UserID).Error == nil {
		out["userName"] = strings.TrimSpace(u.FirstName + " " + u.LastName)
		// Même rappel que celui de design.html : le shop est servi par la SPA,
		// et c'est là que l'adhérent passe le plus clair de son temps. Le path
		// passé est vide, et non celui de cette requête : l'exclusion porte sur
		// la page d'édition du compte, que Go sert lui-même — aucune route de
		// la SPA n'a à s'en exclure.
		out["suggestPhone"] = suggestPhone(&u, "")
	}

	if claims.ImpersonatorID != 0 {
		out["impersonating"] = true
		var imp model.User
		if h.db.First(&imp, claims.ImpersonatorID).Error == nil {
			out["impersonatorName"] = strings.TrimSpace(imp.FirstName + " " + imp.LastName)
		}
	}

	c.JSON(http.StatusOK, out)
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
		// userId : un admin du groupe peut passer commande pour un membre.
		UserID uint `json:"userId"`
		Orders []struct {
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
	if err := h.db.Preload("Catalog").Preload("MultiDistrib").
		Where("multi_distrib_id = ? AND catalog_id = ?", mdID, body.CatalogID).
		First(&distrib).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution not found"})
		return
	}

	// Résolution du destinataire : par défaut l'utilisateur courant ; si userId
	// est fourni et différent, vérifier que l'appelant est gestionnaire du groupe.
	targetID := claims.UserID
	isManager := false
	if body.UserID != 0 && body.UserID != claims.UserID {
		ug := loadGroupAccess(h.db, claims.UserID, distrib.Catalog.GroupID)
		if ug == nil || !ug.IsGroupManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can edit orders for other users"})
			return
		}
		isManager = true
		targetID = body.UserID
	}

	// La clôture ne tenait qu'à l'affichage : le formulaire disparaissait, mais
	// la requête passait encore. Un membre qui garde son panier ouvert par-delà
	// l'heure de fermeture, ou qui rejoue l'appel, commandait sans obstacle.
	//
	// Un gestionnaire qui saisit pour un membre est dispensé de la clôture —
	// rattraper une commande après coup fait partie de son travail — mais pas
	// de l'ouverture : avant elle, il n'y a rien à rattraper.
	if !distrib.CanOrderNow() {
		switch {
		case !isManager:
			c.JSON(http.StatusForbidden, gin.H{"error": "les commandes de ce catalogue sont fermées"})
			return
		case !distrib.OrderWindowStarted(time.Now()):
			c.JSON(http.StatusForbidden, gin.H{"error": "les commandes de ce catalogue ne sont pas encore ouvertes"})
			return
		}
	}

	// Delete existing orders for this user + distribution
	h.db.Where("user_id = ? AND distribution_id = ?", targetID, distrib.ID).
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
			UserID:         targetID,
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
