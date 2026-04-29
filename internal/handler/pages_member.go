package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/internal/service"
	"github.com/gpenaud/alterconso/pkg/mailer"
)

// ---- View types ----

type MemberDetailData struct {
	PageData
	Member          model.User
	MemberUG        model.UserGroup
	CatalogSubs     []CatalogSubsView
	DistribOrderSets []DistribOrderSet
}

type CatalogSubsView struct {
	CatalogName string
	CatalogID   uint
	Subs        []SubDetailView
}

type SubDetailView struct {
	ID        uint
	StartDate string
	EndDate   string
	Total     float64
	Paid      bool
}

type DistribOrderSet struct {
	Date   string
	Orders []OrderLineView
	Total  float64
}

type OrderLineView struct {
	ProductName string
	SmartQty    string
	ProductPrice float64
	SubTotal    float64
	Fees        float64
	Total       float64
	CatalogName string
	CatalogID   uint
	Paid        bool
}

type MemberBalanceEntry struct {
	ID        uint
	FirstName string
	LastName  string
	Email     string
	Balance   float64
}

// ---- /member/view/:id ----

func (h *PagesHandler) MemberViewPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var member model.User
	if err := h.db.First(&member, id).Error; err != nil {
		c.String(http.StatusNotFound, "membre introuvable")
		return
	}

	var ug model.UserGroup
	h.db.Where("user_id = ? AND group_id = ?", id, pd.Group.ID).First(&ug)

	// Subscriptions by catalog
	var subs []model.Subscription
	h.db.Where("user_id = ?", id).Preload("Catalog").Find(&subs)

	// Group by catalog
	catalogMap := make(map[uint]*CatalogSubsView)
	for _, s := range subs {
		if s.Catalog.GroupID != pd.Group.ID {
			continue
		}
		if _, ok := catalogMap[s.CatalogID]; !ok {
			catalogMap[s.CatalogID] = &CatalogSubsView{
				CatalogName: s.Catalog.Name,
				CatalogID:   s.CatalogID,
			}
		}
		sd := SubDetailView{
			ID:        s.ID,
			StartDate: s.StartDate.Format("02/01/2006"),
		}
		if s.EndDate != nil {
			sd.EndDate = s.EndDate.Format("02/01/2006")
		}
		catalogMap[s.CatalogID].Subs = append(catalogMap[s.CatalogID].Subs, sd)
	}

	// Upcoming variable orders grouped by distribution
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var orders []model.UserOrder
	h.db.Where("user_orders.user_id = ?", id).
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
		Where("multi_distribs.group_id = ?", pd.Group.ID).
		Where("multi_distribs.distrib_start_date >= ?", today).
		Preload("Product").
		Preload("Product.Catalog").
		Preload("Distribution").
		Preload("Distribution.MultiDistrib").
		Order("multi_distribs.distrib_start_date ASC").
		Limit(100).
		Find(&orders)

	distribMap := make(map[uint]*DistribOrderSet)
	distribOrder := []uint{}
	for _, o := range orders {
		if o.Distribution == nil {
			continue
		}
		mdID := o.Distribution.MultiDistribID
		if _, ok := distribMap[mdID]; !ok {
			distribMap[mdID] = &DistribOrderSet{
				Date: o.Distribution.MultiDistrib.DistribStartDate.Format("02/01/2006"),
			}
			distribOrder = append(distribOrder, mdID)
		}
		fees := o.TotalPrice() - o.Quantity*o.ProductPrice
		line := OrderLineView{
			ProductName:  o.Product.Name,
			SmartQty:     formatQty(o.Quantity, o.Product.UnitType),
			ProductPrice: o.ProductPrice,
			SubTotal:     o.Quantity * o.ProductPrice,
			Fees:         fees,
			Total:        o.TotalPrice(),
			CatalogName:  o.Product.Catalog.Name,
			CatalogID:    o.Product.CatalogID,
			Paid:         o.Paid,
		}
		distribMap[mdID].Orders = append(distribMap[mdID].Orders, line)
		distribMap[mdID].Total += o.TotalPrice()
	}

	ddata := MemberDetailData{PageData: pd}
	ddata.Member = member
	ddata.MemberUG = ug
	ddata.Title = member.FirstName + " " + member.LastName

	for _, c := range catalogMap {
		ddata.CatalogSubs = append(ddata.CatalogSubs, *c)
	}
	for _, mdID := range distribOrder {
		ddata.DistribOrderSets = append(ddata.DistribOrderSets, *distribMap[mdID])
	}

	t, err := loadTemplates("base.html", "design.html", "member_view.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", ddata); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /member/payments/:id ----

type MemberPaymentsData struct {
	PageData
	Member  model.User
	Balance float64
	Ops     []OperationView
	Page    int
	Pages   int
}

type OperationView struct {
	ID          uint
	Date        string
	Type        string
	Description string
	Amount      float64
	PaymentType string
}

func (h *PagesHandler) MemberPaymentsPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var member model.User
	if err := h.db.First(&member, id).Error; err != nil {
		c.String(http.StatusNotFound, "membre introuvable")
		return
	}

	var ug model.UserGroup
	h.db.Where("user_id = ? AND group_id = ?", id, pd.Group.ID).First(&ug)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage := 20

	var total int64
	h.db.Model(&model.Operation{}).Where("user_id = ? AND group_id = ?", id, pd.Group.ID).Count(&total)

	var ops []model.Operation
	h.db.Where("user_id = ? AND group_id = ?", id, pd.Group.ID).
		Order("created_at DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&ops)

	opViews := make([]OperationView, 0, len(ops))
	for _, op := range ops {
		desc := ""
		if op.Description != nil {
			desc = *op.Description
		}
		pt := ""
		if op.PaymentType != nil {
			pt = *op.PaymentType
		}
		opViews = append(opViews, OperationView{
			ID:          op.ID,
			Date:        op.CreatedAt.Format("02/01/2006 15:04"),
			Type:        op.Type,
			Description: desc,
			Amount:      op.Amount,
			PaymentType: pt,
		})
	}

	pages := int(total)/perPage + 1
	if int(total)%perPage == 0 && total > 0 {
		pages = int(total) / perPage
	}

	data := MemberPaymentsData{
		PageData: pd,
		Member:   member,
		Balance:  ug.Balance,
		Ops:      opViews,
		Page:     page,
		Pages:    pages,
	}
	data.Title = "Paiements — " + member.FirstName + " " + member.LastName

	t, err := loadTemplates("base.html", "design.html", "member_payments.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /member/balance ----

func (h *PagesHandler) MemberBalancePage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	var ugs []model.UserGroup
	h.db.Where("group_id = ?", pd.Group.ID).Preload("User").Find(&ugs)

	type BalancePage struct {
		PageData
		Entries []MemberBalanceEntry
	}
	bp := BalancePage{PageData: pd}
	bp.Title = "Soldes des membres"

	for _, ug := range ugs {
		bp.Entries = append(bp.Entries, MemberBalanceEntry{
			ID:        ug.User.ID,
			FirstName: ug.User.FirstName,
			LastName:  ug.User.LastName,
			Email:     ug.User.Email,
			Balance:   ug.Balance,
		})
	}

	t, err := loadTemplates("base.html", "design.html", "member_balance.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", bp); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /member/insert ----

func (h *PagesHandler) MemberInsertPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	type InsertPage struct {
		PageData
		Flash string
		Error string
	}
	ip := InsertPage{PageData: pd}
	ip.Title = "Ajouter un membre"

	if c.Request.Method == http.MethodPost {
		email := strings.TrimSpace(c.PostForm("email"))
		firstName := strings.TrimSpace(c.PostForm("firstName"))
		lastName := strings.TrimSpace(c.PostForm("lastName"))

		if email == "" || firstName == "" || lastName == "" {
			ip.Error = "Prénom, nom et email sont requis."
		} else {
			var user model.User
			if err := h.db.Where("email = ?", email).First(&user).Error; err != nil {
				user = model.User{
					Email:     email,
					FirstName: firstName,
					LastName:  lastName,
				}
				// Optional fields
				if v := strings.TrimSpace(c.PostForm("phone")); v != "" {
					user.Phone = &v
				}
				if v := strings.TrimSpace(c.PostForm("firstName2")); v != "" {
					user.FirstName2 = &v
				}
				if v := strings.TrimSpace(c.PostForm("lastName2")); v != "" {
					user.LastName2 = &v
				}
				if v := strings.TrimSpace(c.PostForm("email2")); v != "" {
					user.Email2 = &v
				}
				if v := strings.TrimSpace(c.PostForm("phone2")); v != "" {
					user.Phone2 = &v
				}
				if v := strings.TrimSpace(c.PostForm("address1")); v != "" {
					user.Address1 = &v
				}
				if v := strings.TrimSpace(c.PostForm("address2")); v != "" {
					user.Address2 = &v
				}
				if v := strings.TrimSpace(c.PostForm("zipCode")); v != "" {
					user.ZipCode = &v
				}
				if v := strings.TrimSpace(c.PostForm("city")); v != "" {
					user.City = &v
				}
				if v := strings.TrimSpace(c.PostForm("birthDate")); v != "" {
					if t, err := time.Parse("2006-01-02", v); err == nil {
						user.BirthDate = &t
					}
				}
				if v := strings.TrimSpace(c.PostForm("nationality")); v != "" && v != "-" {
					user.Nationality = &v
				}
				if v := strings.TrimSpace(c.PostForm("countryOfResidence")); v != "" && v != "-" {
					user.CountryOfResidence = &v
				}
				// Notification flags
				var flags model.UserFlag
				if c.PostForm("notif4h") != "" {
					flags |= model.UserFlagEmailNotif4h
				}
				if c.PostForm("notif24h") != "" {
					flags |= model.UserFlagEmailNotif24h
				}
				if c.PostForm("notifOpen") != "" {
					flags |= model.UserFlagEmailNotifOuverture
				}
				user.Flags = uint(flags)
				h.db.Create(&user)
			}
			var existing model.UserGroup
			if h.db.Where("user_id = ? AND group_id = ?", user.ID, pd.Group.ID).First(&existing).Error == nil {
				ip.Error = "Cette personne est déjà membre du groupe."
			} else {
				h.db.Create(&model.UserGroup{UserID: user.ID, GroupID: pd.Group.ID})
				c.Redirect(http.StatusFound, "/member")
				return
			}
		}
	}

	t, err := loadTemplates("base.html", "design.html", "member_insert.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", ip); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /member/edit/:id ----

type CountryOption struct {
	Code  string
	Label string
}

var nationalities = []CountryOption{
	{"AF", "Afghane"}, {"DE", "Allemande"}, {"DZ", "Algérienne"}, {"BE", "Belge"},
	{"BR", "Brésilienne"}, {"CA", "Canadienne"}, {"CN", "Chinoise"}, {"HR", "Croate"},
	{"DK", "Danoise"}, {"EG", "Égyptienne"}, {"ES", "Espagnole"}, {"US", "Américaine"},
	{"FI", "Finlandaise"}, {"FR", "Française"}, {"GB", "Britannique"}, {"GR", "Grecque"},
	{"HU", "Hongroise"}, {"IN", "Indienne"}, {"IE", "Irlandaise"}, {"IT", "Italienne"},
	{"JP", "Japonaise"}, {"LB", "Libanaise"}, {"LU", "Luxembourgeoise"}, {"MA", "Marocaine"},
	{"MX", "Mexicaine"}, {"NL", "Néerlandaise"}, {"NO", "Norvégienne"}, {"PL", "Polonaise"},
	{"PT", "Portugaise"}, {"RO", "Roumaine"}, {"RU", "Russe"}, {"SE", "Suédoise"},
	{"CH", "Suisse"}, {"TN", "Tunisienne"}, {"TR", "Turque"}, {"UA", "Ukrainienne"},
}

var countries = []CountryOption{
	{"DE", "Allemagne"}, {"DZ", "Algérie"}, {"BE", "Belgique"}, {"BR", "Brésil"},
	{"CA", "Canada"}, {"CN", "Chine"}, {"HR", "Croatie"}, {"DK", "Danemark"},
	{"EG", "Égypte"}, {"ES", "Espagne"}, {"US", "États-Unis"}, {"FI", "Finlande"},
	{"FR", "France"}, {"GB", "Royaume-Uni"}, {"GR", "Grèce"}, {"HU", "Hongrie"},
	{"IN", "Inde"}, {"IE", "Irlande"}, {"IT", "Italie"}, {"JP", "Japon"},
	{"LB", "Liban"}, {"LU", "Luxembourg"}, {"MA", "Maroc"}, {"MX", "Mexique"},
	{"NL", "Pays-Bas"}, {"NO", "Norvège"}, {"PL", "Pologne"}, {"PT", "Portugal"},
	{"RO", "Roumanie"}, {"RU", "Russie"}, {"SE", "Suède"}, {"CH", "Suisse"},
	{"TN", "Tunisie"}, {"TR", "Turquie"}, {"UA", "Ukraine"},
}

func (h *PagesHandler) MemberEditPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var member model.User
	if err := h.db.First(&member, id).Error; err != nil {
		c.String(http.StatusNotFound, "membre introuvable")
		return
	}

	if c.Request.Method == http.MethodPost {
		strPtr := func(s string) *string {
			if s == "" { return nil }
			return &s
		}
		var flags uint
		if c.PostForm("notif4h") == "1"   { flags |= uint(model.UserFlagEmailNotif4h) }
		if c.PostForm("notif24h") == "1"  { flags |= uint(model.UserFlagEmailNotif24h) }
		if c.PostForm("notifOpen") == "1" { flags |= uint(model.UserFlagEmailNotifOuverture) }

		updates := map[string]interface{}{
			"first_name":           strings.TrimSpace(c.PostForm("firstName")),
			"last_name":            strings.TrimSpace(c.PostForm("lastName")),
			"phone":                strPtr(strings.TrimSpace(c.PostForm("phone"))),
			"first_name2":          strPtr(strings.TrimSpace(c.PostForm("firstName2"))),
			"last_name2":           strPtr(strings.TrimSpace(c.PostForm("lastName2"))),
			"email2":               strPtr(strings.TrimSpace(c.PostForm("email2"))),
			"phone2":               strPtr(strings.TrimSpace(c.PostForm("phone2"))),
			"address1":             strPtr(strings.TrimSpace(c.PostForm("address1"))),
			"address2":             strPtr(strings.TrimSpace(c.PostForm("address2"))),
			"zip_code":             strPtr(strings.TrimSpace(c.PostForm("zipCode"))),
			"city":                 strPtr(strings.TrimSpace(c.PostForm("city"))),
			"nationality":          strPtr(c.PostForm("nationality")),
			"country_of_residence": strPtr(c.PostForm("countryOfResidence")),
			"flags":                flags,
		}
		if bd := strings.TrimSpace(c.PostForm("birthDate")); bd != "" {
			if t, err := time.Parse("2006-01-02", bd); err == nil {
				updates["birth_date"] = t
			}
		} else {
			updates["birth_date"] = nil
		}
		// Réinitialisation du mot de passe (admins seulement)
		if pd.IsGroupManager || pd.HasMembership {
			if newPass := strings.TrimSpace(c.PostForm("newPassword")); newPass != "" {
				u := model.User{ID: uint(id)}
				u.SetPassword(newPass, h.cfg.Key)
				updates["pass"] = u.Pass
			}
		}
		h.db.Model(&model.User{}).Where("id = ?", id).Updates(updates)
		c.Redirect(http.StatusFound, "/member/view/"+c.Param("id"))
		return
	}

	type EditPage struct {
		PageData
		Member        model.User
		Nationalities []CountryOption
		Countries     []CountryOption
		Notif4h       bool
		Notif24h      bool
		NotifOpen     bool
	}
	ep := EditPage{
		PageData:      pd,
		Member:        member,
		Nationalities: nationalities,
		Countries:     countries,
		Notif4h:       member.HasFlag(model.UserFlagEmailNotif4h),
		Notif24h:      member.HasFlag(model.UserFlagEmailNotif24h),
		NotifOpen:     member.HasFlag(model.UserFlagEmailNotifOuverture),
	}
	ep.Title = "Modifier — " + member.FirstName + " " + member.LastName

	t, err := loadTemplates("base.html", "design.html", "member_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", ep); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET /member/delete/:id (retirer du groupe courant) ----

func (h *PagesHandler) MemberDelete(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	if isSiteAdmin(h.db, uint(id)) {
		c.String(http.StatusForbidden, "le superadmin global ne peut pas être retiré")
		return
	}
	h.db.Where("user_id = ? AND group_id = ?", id, pd.Group.ID).Delete(&model.UserGroup{})
	c.Redirect(http.StatusFound, "/member")
}

// ---- POST /member/fullDelete/:id (supprimer le compte de la base) ----

func (h *PagesHandler) MemberFullDelete(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.Redirect(http.StatusFound, "/home")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	if uint(id) == pd.User.ID {
		c.String(http.StatusBadRequest, "vous ne pouvez pas supprimer votre propre compte")
		return
	}
	if isSiteAdmin(h.db, uint(id)) {
		c.String(http.StatusForbidden, "le superadmin global ne peut pas être supprimé")
		return
	}
	uid := uint(id)

	// Suppression en cascade des données rattachées au user
	h.db.Where("user_id = ?", uid).Delete(&model.UserGroup{})
	h.db.Where("user_id = ?", uid).Delete(&model.UserOrder{})
	h.db.Where("user_id = ?", uid).Delete(&model.Volunteer{})
	h.db.Where("user_id = ?", uid).Delete(&model.Subscription{})
	h.db.Where("user_id = ?", uid).Delete(&model.Membership{})
	h.db.Where("user_id = ?", uid).Delete(&model.WaitingList{})
	h.db.Where("user_id = ?", uid).Delete(&model.Basket{})
	h.db.Where("user_id = ?", uid).Delete(&model.PasswordResetToken{})
	h.db.Where("user_id = ?", uid).Delete(&model.EmailVerifyToken{})
	h.db.Where("sender_id = ?", uid).Delete(&model.Message{})

	// Suppression du user
	h.db.Delete(&model.User{}, uid)

	c.Redirect(http.StatusFound, "/member")
}

// ---- POST /transaction/insertPayment/:memberId ----

type InsertPaymentData struct {
	PageData
	Member  model.User
	Balance float64
}

func (h *PagesHandler) InsertPaymentPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || (!pd.IsGroupManager && !pd.HasMembership) {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	id, err := strconv.ParseUint(c.Param("memberId"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}

	var member model.User
	if err := h.db.First(&member, id).Error; err != nil {
		c.String(http.StatusNotFound, "membre introuvable")
		return
	}

	var ug model.UserGroup
	h.db.Where("user_id = ? AND group_id = ?", id, pd.Group.ID).First(&ug)

	if c.Request.Method == http.MethodPost {
		amount, _ := strconv.ParseFloat(c.PostForm("amount"), 64)
		desc := strings.TrimSpace(c.PostForm("description"))
		paymentType := strings.TrimSpace(c.PostForm("paymentType"))

		if amount != 0 {
			op := model.Operation{
				UserID:      uint(id),
				GroupID:     pd.Group.ID,
				Amount:      amount,
				Type:        "Payment",
				PaymentType: &paymentType,
				Pending:     false,
			}
			if desc != "" {
				op.Description = &desc
			}
			h.db.Create(&op)
			// Update balance
			h.db.Model(&model.UserGroup{}).
				Where("user_id = ? AND group_id = ?", id, pd.Group.ID).
				UpdateColumn("balance", h.db.Raw("balance + ?", amount))
		}
		c.Redirect(http.StatusFound, "/member/payments/"+c.Param("memberId"))
		return
	}

	data := InsertPaymentData{
		PageData: pd,
		Member:   member,
		Balance:  ug.Balance,
	}
	data.Title = "Saisir un paiement"
	data.Category = "member"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Membres", Link: "/member"}}

	t, err := loadTemplates("base.html", "design.html", "insert_payment.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET /user/forgottenPassword ----

type ForgotPasswordData struct {
	PageData
	Step  int
	Error string
}

func (h *PagesHandler) ForgotPasswordPage(c *gin.Context) {
	data := ForgotPasswordData{Step: 1}

	if c.Request.Method == http.MethodPost {
		email := strings.TrimSpace(c.PostForm("email"))
		var user model.User
		if err := h.db.Where("email = ?", strings.ToLower(email)).First(&user).Error; err != nil {
			data.Error = "Aucun compte n'est associé à cette adresse email."
		} else {
			// Supprimer les anciens tokens de cet utilisateur
			h.db.Where("user_id = ?", user.ID).Delete(&model.PasswordResetToken{})

			// Générer un token sécurisé
			tokenBytes := make([]byte, 32)
			rand.Read(tokenBytes)
			token := hex.EncodeToString(tokenBytes)

			h.db.Create(&model.PasswordResetToken{
				UserID:    user.ID,
				Token:     token,
				ExpiresAt: time.Now().Add(1 * time.Hour),
			})

			// Envoyer l'email
			resetURL := fmt.Sprintf("https://%s/user/resetPassword?token=%s", h.cfg.Host, token)
			h.sendPasswordResetEmail(user, resetURL)
			data.Step = 2
		}
	}

	t, err := loadTemplates("base.html", "design.html", "forgotten_password.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (h *PagesHandler) sendPasswordResetEmail(user model.User, resetURL string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr"><head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;background:#f5f0e8;padding:24px;">
<table width="100%%" cellpadding="0" cellspacing="0">
  <tr><td align="center">
    <table width="560" style="background:#fff;border-radius:4px;overflow:hidden;">
      <tr><td style="background:#6a9a2a;padding:20px 30px;">
        <h1 style="margin:0;color:#fff;font-size:1.3em;">Réinitialisation de mot de passe</h1>
      </td></tr>
      <tr><td style="padding:28px 30px;">
        <p>Bonjour <strong>%s</strong>,</p>
        <p>Vous avez demandé la réinitialisation de votre mot de passe. Cliquez sur le bouton ci-dessous pour en définir un nouveau :</p>
        <table cellpadding="0" cellspacing="0" style="margin:24px 0;">
          <tr><td style="background:#6a9a2a;border-radius:4px;">
            <a href="%s" style="display:inline-block;padding:12px 28px;color:#fff;text-decoration:none;font-weight:bold;">
              Réinitialiser mon mot de passe →
            </a>
          </td></tr>
        </table>
        <p style="color:#888;font-size:0.85em;">Ce lien est valable <strong>1 heure</strong>. Si vous n'êtes pas à l'origine de cette demande, ignorez cet email.</p>
      </td></tr>
    </table>
  </td></tr>
</table>
</body></html>`, user.FirstName, resetURL)

	m := &mailer.Mail{
		From:     h.cfg.DefaultEmail,
		FromName: "Alterconso",
		Subject:  "Réinitialisation de votre mot de passe",
		HTMLBody: html,
	}
	m.AddRecipient(user.Email, user.FirstName+" "+user.LastName)
	if err := h.mailer.Send(m); err != nil {
		fmt.Printf("[MAIL] password reset failed for %s: %v\n", user.Email, err)
	}
}

// ---- GET/POST /user/register ----

type RegisterData struct {
	PageData
	Step      int
	Error     string
	Email     string
	FirstName string
	LastName  string
}

func (h *PagesHandler) RegisterPage(c *gin.Context) {
	data := RegisterData{Step: 1}

	if c.Request.Method == http.MethodPost {
		email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
		password := c.PostForm("password")
		passwordConfirm := c.PostForm("passwordConfirm")
		firstName := strings.TrimSpace(c.PostForm("firstName"))
		lastName := strings.TrimSpace(c.PostForm("lastName"))

		data.Email = email
		data.FirstName = firstName
		data.LastName = lastName

		switch {
		case email == "" || firstName == "" || lastName == "" || password == "":
			data.Error = "Tous les champs sont requis."
		case len(password) < 8:
			data.Error = "Le mot de passe doit faire au moins 8 caractères."
		case password != passwordConfirm:
			data.Error = "Les deux mots de passe ne correspondent pas."
		default:
			var existing model.User
			if err := h.db.Where("email = ?", email).First(&existing).Error; err == nil {
				if existing.EmailVerifiedAt != nil {
					data.Error = "Cet email est déjà utilisé."
				} else {
					// Compte non vérifié : on régénère un token et renvoie un email
					h.db.Where("user_id = ?", existing.ID).Delete(&model.EmailVerifyToken{})
					token := newSecureToken()
					h.db.Create(&model.EmailVerifyToken{
						UserID: existing.ID, Token: token,
						ExpiresAt: time.Now().Add(24 * time.Hour),
					})
					url := fmt.Sprintf("https://%s/user/verify?token=%s", h.cfg.Host, token)
					h.sendVerifyEmail(existing, url)
					data.Step = 2
					break
				}
				break
			}

			user := model.User{Email: email, FirstName: firstName, LastName: lastName}
			user.SetPassword(password, h.cfg.Key)
			if err := h.db.Create(&user).Error; err != nil {
				data.Error = "Erreur lors de la création du compte."
				break
			}
			token := newSecureToken()
			h.db.Create(&model.EmailVerifyToken{
				UserID: user.ID, Token: token,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			})
			url := fmt.Sprintf("https://%s/user/verify?token=%s", h.cfg.Host, token)
			h.sendVerifyEmail(user, url)
			data.Step = 2
		}
	}

	t, err := loadTemplates("base.html", "register.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// GET /user/verify?token=...
func (h *PagesHandler) VerifyEmailPage(c *gin.Context) {
	token := c.Query("token")
	type verifyData struct {
		PageData
		OK    bool
		Error string
	}
	data := verifyData{}

	if token == "" {
		data.Error = "Lien d'activation invalide."
	} else {
		var t model.EmailVerifyToken
		if err := h.db.Where("token = ?", token).First(&t).Error; err != nil {
			data.Error = "Ce lien d'activation est invalide ou déjà utilisé."
		} else if t.IsExpired() {
			data.Error = "Ce lien d'activation a expiré. Inscrivez-vous à nouveau."
			h.db.Delete(&t)
		} else {
			now := time.Now()
			h.db.Model(&model.User{}).Where("id = ?", t.UserID).Update("email_verified_at", now)
			h.db.Delete(&t)

			// Auto-ajout aux groupes en mode d'inscription "Ouvert"
			var autoGroups []model.Group
			h.db.Where("reg_option = ?", string(model.RegOptionOpen)).Find(&autoGroups)
			for _, g := range autoGroups {
				var existing model.UserGroup
				if err := h.db.Where("user_id = ? AND group_id = ?", t.UserID, g.ID).First(&existing).Error; err == nil {
					continue // déjà membre
				}
				h.db.Create(&model.UserGroup{
					UserID:  t.UserID,
					GroupID: g.ID,
					Rights:  "[]",
				})
			}

			// Auto-login : émission d'un JWT et cookie, puis redirection vers la complétion du profil
			if jwtToken, err := h.issueToken(t.UserID, 0); err == nil {
				c.SetCookie("token", jwtToken, 3600*24*7, "/", "", false, true)
				c.Redirect(http.StatusFound, "/user/completeProfile?welcome=1")
				return
			}
			data.OK = true
		}
	}

	tpl, err := loadTemplates("base.html", "register_verify.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := tpl.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func newSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *PagesHandler) sendVerifyEmail(user model.User, url string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr"><head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;background:#f5f0e8;padding:24px;">
<table width="100%%" cellpadding="0" cellspacing="0">
  <tr><td align="center">
    <table width="560" style="background:#fff;border-radius:4px;overflow:hidden;">
      <tr><td style="background:#6a9a2a;padding:20px 30px;">
        <h1 style="margin:0;color:#fff;font-size:1.3em;">Bienvenue sur Alterconso</h1>
      </td></tr>
      <tr><td style="padding:28px 30px;">
        <p>Bonjour <strong>%s</strong>,</p>
        <p>Merci de votre inscription. Pour activer votre compte, cliquez sur le bouton ci-dessous :</p>
        <table cellpadding="0" cellspacing="0" style="margin:24px 0;">
          <tr><td style="background:#6a9a2a;border-radius:4px;">
            <a href="%s" style="display:inline-block;padding:12px 28px;color:#fff;text-decoration:none;font-weight:bold;">
              Activer mon compte →
            </a>
          </td></tr>
        </table>
        <p style="color:#888;font-size:0.85em;">Ce lien est valable <strong>24 heures</strong>. Si vous n'êtes pas à l'origine de cette inscription, ignorez cet email.</p>
      </td></tr>
    </table>
  </td></tr>
</table>
</body></html>`, user.FirstName, url)

	m := &mailer.Mail{
		From:     h.cfg.DefaultEmail,
		FromName: "Alterconso",
		Subject:  "Activation de votre compte Alterconso",
		HTMLBody: html,
	}
	m.AddRecipient(user.Email, user.FirstName+" "+user.LastName)
	if err := h.mailer.Send(m); err != nil {
		fmt.Printf("[MAIL] verify failed for %s: %v\n", user.Email, err)
	}
}

// ---- GET/POST /user/completeProfile ----
// Affichée après activation du compte pour collecter les infos optionnelles.

type CompleteProfileData struct {
	PageData
	Member        *model.User
	Nationalities []CountryOption
	Countries     []CountryOption
	Welcome       bool
	Saved         bool
	Error         string
}

func (h *PagesHandler) CompleteProfilePage(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.Redirect(http.StatusFound, "/user/login")
		return
	}

	data := CompleteProfileData{
		Member:        &user,
		Nationalities: nationalities,
		Countries:     countries,
		Welcome:       c.Query("welcome") == "1",
	}
	data.Title = "Compléter mon profil"

	if c.Request.Method == http.MethodPost {
		strPtr := func(s string) *string {
			if s == "" {
				return nil
			}
			return &s
		}
		updates := map[string]interface{}{
			"phone":                strPtr(strings.TrimSpace(c.PostForm("phone"))),
			"address1":             strPtr(strings.TrimSpace(c.PostForm("address1"))),
			"address2":             strPtr(strings.TrimSpace(c.PostForm("address2"))),
			"zip_code":             strPtr(strings.TrimSpace(c.PostForm("zipCode"))),
			"city":                 strPtr(strings.TrimSpace(c.PostForm("city"))),
			"nationality":          strPtr(c.PostForm("nationality")),
			"country_of_residence": strPtr(c.PostForm("countryOfResidence")),
		}
		if bd := strings.TrimSpace(c.PostForm("birthDate")); bd != "" {
			if t, err := time.Parse("2006-01-02", bd); err == nil {
				updates["birth_date"] = t
			}
		}
		h.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates)
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	t, err := loadTemplates("base.html", "complete_profile.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /user/definePassword ----

type DefinePasswordData struct {
	PageData
	Step  int
	Error string
	Token string
}

func (h *PagesHandler) DefinePasswordPage(c *gin.Context) {
	data := DefinePasswordData{Step: 3}
	token := c.Query("token")
	if token == "" {
		token = c.PostForm("token")
	}
	data.Token = token

	// Valider le token
	var resetToken model.PasswordResetToken
	if err := h.db.Where("token = ?", token).First(&resetToken).Error; err != nil {
		data.Error = "Lien invalide ou expiré."
		data.Step = 0
	} else if resetToken.IsExpired() {
		h.db.Delete(&resetToken)
		data.Error = "Ce lien a expiré. Veuillez faire une nouvelle demande."
		data.Step = 0
	}

	if c.Request.Method == http.MethodPost && data.Step == 3 {
		pass := c.PostForm("password")
		pass2 := c.PostForm("password2")
		if pass == "" || pass != pass2 {
			data.Error = "Les mots de passe ne correspondent pas."
		} else {
			var user model.User
			if h.db.First(&user, resetToken.UserID).Error == nil {
				user.SetPassword(pass, h.cfg.Key)
				h.db.Model(&user).Update("pass", user.Pass)
				// Invalider le token après utilisation
				h.db.Delete(&resetToken)
				data.Step = 4
			} else {
				data.Error = "Utilisateur introuvable."
			}
		}
	}

	t, err := loadTemplates("base.html", "design.html", "define_password.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /messages ----

type MessagesData struct {
	PageData
	SentMessages   []MessageView
	Today          string
	BrevoLimit     int
	BrevoRemaining int
	BrevoError     string
	CountAll       int
	CountMembers   int
	CountManagers  int
	// Catégories d'activité issues de la config (mutuellement exclusives,
	// dans l'ordre de priorité du fichier YAML).
	ActivityCategories []ActivityCategoryView
	// RecipientEmailsJSON : objet JS littéral { "all": [...], "managers": [...], ... }
	// inliné brut dans un <script> via template.JS (pas de re-encodage).
	RecipientEmailsJSON template.JS
	// TempRecipient : destinataire éphémère injecté via query string, non
	// persistant. Disparaît dès qu'on recharge /messages sans le param.
	// Cas d'usage actuel : ?distribOrders=YYYY-MM-DD pour les clients d'une
	// distribution donnée.
	TempRecipient *RecipientOption
	// Feedback d'envoi affiché après un POST réussi (PRG via query params).
	SendSuccess  int
	SendFailed   int
	SendNoRcpt   bool
}

// RecipientOption décrit une option ajoutée dynamiquement au select des
// destinataires (clé d'identification, label affiché, compteur, tooltip).
type RecipientOption struct {
	Value   string
	Name    string
	Tooltip string
	Count   int
}

// ActivityCategoryView est une catégorie d'activité prête à afficher.
// Value est utilisée comme valeur du <option> et clé pour le compteur JS.
// Tooltip est le texte natif title="" : formule + lignes "Inclut: ...".
type ActivityCategoryView struct {
	Value   string
	Name    string
	Tooltip string
	Count   int
}

type MessageView struct {
	ID      uint
	Title   string
	Date    string
	Body    string
}

// buildRecipientEmails retourne les emails par valeur de destinataire :
// "all", "managers", "members" pour les catégories fixes (rôle), et
// "activity-N" pour chaque catégorie d'activité (mutuellement exclusives).
// distribOrdersEmails retourne les emails (uniques, non vides) des membres
// du groupe ayant commandé sur une distribution programmée au jour donné.
// Utilisé pour le destinataire éphémère "Clients de la commande du DD/MM/YYYY".
func (h *PagesHandler) distribOrdersEmails(groupID uint, day time.Time) []string {
	dayStr := day.Format("2006-01-02")
	var emails []string
	h.db.Raw(`
		SELECT DISTINCT u.email
		FROM users u
		JOIN user_orders uo ON uo.user_id = u.id
		JOIN distributions d ON d.id = uo.distribution_id
		JOIN multi_distribs md ON md.id = d.multi_distrib_id
		WHERE md.group_id = ? AND DATE(md.distrib_start_date) = ? AND u.email <> ''
		ORDER BY u.last_name, u.first_name
	`, groupID, dayStr).Scan(&emails)
	return emails
}

// Utilisé pour alimenter le tooltip de la page /messages.
func (h *PagesHandler) buildRecipientEmails(groupID uint, now time.Time) map[string][]string {
	var ugs []model.UserGroup
	h.db.
		Preload("User").
		Joins("JOIN users ON users.id = user_groups.user_id").
		Where("user_groups.group_id = ?", groupID).
		Order("users.last_name, users.first_name").
		Find(&ugs)

	out := map[string][]string{
		"all":      {},
		"managers": {},
		"members":  {},
	}
	memberIDs := make([]uint, 0, len(ugs))
	emailByID := make(map[uint]string, len(ugs))
	for _, ug := range ugs {
		email := ug.User.Email
		if email == "" {
			continue
		}
		out["all"] = append(out["all"], email)
		if strings.Contains(ug.Rights, "GroupAdmin") {
			out["managers"] = append(out["managers"], email)
		} else {
			out["members"] = append(out["members"], email)
		}
		memberIDs = append(memberIDs, ug.UserID)
		emailByID[ug.UserID] = email
	}

	cats := h.cfg.Messages.RecipientCategories
	for i := range cats {
		out[fmt.Sprintf("activity-%d", i)] = []string{}
	}
	if len(cats) == 0 || len(memberIDs) == 0 {
		return out
	}

	sets := service.BuildCategorySets(h.db, groupID, memberIDs, now, cats)
	for i := range cats {
		key := fmt.Sprintf("activity-%d", i)
		// Préserve l'ordre des memberIDs (déjà trié par last_name, first_name).
		for _, uid := range memberIDs {
			if sets[i][uid] {
				out[key] = append(out[key], emailByID[uid])
			}
		}
	}
	return out
}

// buildCategoryTooltip retourne le texte affiché en title="" sur l'option
// d'une catégorie d'activité : la formule (compact) suivie, le cas échéant,
// d'une ligne "Inclut : Cat1, Cat2, ...".
func buildCategoryTooltip(cat config.RecipientCategory) string {
	tt := cat.Compact()
	if len(cat.Includes) > 0 {
		tt += "\nInclut : " + strings.Join(cat.Includes, ", ")
	}
	return tt
}

// computeActivityCategoryCounts répartit les membres du groupe dans les
// catégories d'activité de la config, en mode mutuellement exclusif :
// chaque user est attribué à la PREMIÈRE catégorie qui matche, dans l'ordre
// du fichier YAML.
func (h *PagesHandler) computeActivityCategoryCounts(groupID uint, now time.Time) []ActivityCategoryView {
	cats := h.cfg.Messages.RecipientCategories
	views := make([]ActivityCategoryView, len(cats))
	for i, cat := range cats {
		views[i] = ActivityCategoryView{
			Value:   fmt.Sprintf("activity-%d", i),
			Name:    cat.Name,
			Tooltip: buildCategoryTooltip(cat),
		}
	}
	if len(cats) == 0 {
		return views
	}

	// IDs de tous les membres du groupe.
	var memberIDs []uint
	h.db.Model(&model.UserGroup{}).
		Where("group_id = ?", groupID).
		Pluck("user_id", &memberIDs)
	if len(memberIDs) == 0 {
		return views
	}

	// Ensembles finaux par catégorie (own + includes). Les catégories peuvent
	// se chevaucher : un user appartient à toutes les catégories dont il
	// satisfait le pattern OU dont il est inclus via `includes`.
	sets := service.BuildCategorySets(h.db, groupID, memberIDs, now, cats)
	for i := range cats {
		views[i].Count = len(sets[i])
	}
	return views
}

func (h *PagesHandler) MessagesPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	var msgs []model.Message
	h.db.Where("group_id = ?", pd.Group.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&msgs)

	data := MessagesData{PageData: pd}
	data.Title = "Messages"
	for _, m := range msgs {
		data.SentMessages = append(data.SentMessages, MessageView{
			ID:    m.ID,
			Title: m.Subject,
			Date:  m.CreatedAt.Format("02/01/2006 15:04"),
			Body:  m.Body,
		})
	}

	// Date du jour (français)
	frMonths := [...]string{"", "janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	frDays := [...]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}
	now := time.Now()
	data.Today = fmt.Sprintf("%s %d %s %d", frDays[now.Weekday()], now.Day(), frMonths[now.Month()], now.Year())

	// Brevo quota
	q := FetchBrevoQuota(h.cfg.BrevoAPIKey)
	data.BrevoLimit = q.DailyLimit
	data.BrevoRemaining = q.Remaining
	data.BrevoError = q.Error

	// Comptage des destinataires potentiels
	var nbAll, nbManagers int64
	h.db.Model(&model.UserGroup{}).Where("group_id = ?", pd.Group.ID).Count(&nbAll)
	h.db.Model(&model.UserGroup{}).
		Where("group_id = ? AND rights LIKE ?", pd.Group.ID, "%GroupAdmin%").
		Count(&nbManagers)
	data.CountAll = int(nbAll)
	data.CountManagers = int(nbManagers)
	data.CountMembers = int(nbAll) - int(nbManagers)
	if data.CountMembers < 0 {
		data.CountMembers = 0
	}

	// Catégories d'activité (config). Les catégories sont mutuellement
	// exclusives : un user appartient à la première catégorie qui matche
	// dans l'ordre de la config, puis on s'arrête.
	data.ActivityCategories = h.computeActivityCategoryCounts(pd.Group.ID, now)

	// Emails par valeur de destinataire — alimente le tooltip côté UI.
	emailsByRecipient := h.buildRecipientEmails(pd.Group.ID, now)

	// Destinataire éphémère : ?distribOrders=YYYY-MM-DD → tous les membres
	// ayant commandé pour une distribution de cette date dans ce groupe.
	if dateStr := c.Query("distribOrders"); dateStr != "" {
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			emails := h.distribOrdersEmails(pd.Group.ID, d)
			key := "distribOrders-" + dateStr
			emailsByRecipient[key] = emails
			label := fmt.Sprintf("Clients de la commande du %02d/%02d/%d",
				d.Day(), int(d.Month()), d.Year())
			data.TempRecipient = &RecipientOption{
				Value:   key,
				Name:    label,
				Tooltip: label,
				Count:   len(emails),
			}
		}
	}

	if b, err := json.Marshal(emailsByRecipient); err == nil {
		data.RecipientEmailsJSON = template.JS(string(b))
	} else {
		data.RecipientEmailsJSON = "{}"
	}

	if c.Request.Method == http.MethodPost {
		subject := strings.TrimSpace(c.PostForm("subject"))
		body := strings.TrimSpace(c.PostForm("body"))
		recipientsValue := strings.TrimSpace(c.PostForm("recipients"))
		senderName := strings.TrimSpace(c.PostForm("senderName"))
		senderEmail := strings.TrimSpace(c.PostForm("senderEmail"))

		if subject != "" && body != "" {
			// Persist le message (table `messages`).
			msg := model.Message{
				SenderID: pd.User.ID,
				GroupID:  pd.Group.ID,
				Subject:  subject,
				Body:     body,
			}
			h.db.Create(&msg)

			// Résout la liste des destinataires.
			emailsByRecipient := h.buildRecipientEmails(pd.Group.ID, now)
			recipients := emailsByRecipient[recipientsValue]

			if len(recipients) == 0 {
				c.Redirect(http.StatusFound, "/messages?nrcpt=1")
				return
			}

			sent, failed := 0, 0
			for _, to := range recipients {
				m := &mailer.Mail{
					From:     h.cfg.DefaultEmail,
					FromName: senderName,
					ReplyTo:  senderEmail,
					Subject:  subject,
					HTMLBody: body,
				}
				m.AddRecipient(to, "")
				if err := h.mailer.Send(m); err != nil {
					failed++
					fmt.Printf("[MAIL] /messages send failed to %s: %v\n", to, err)
				} else {
					sent++
				}
			}
			fmt.Printf("[MAIL] /messages subject=%q sent=%d failed=%d\n", subject, sent, failed)

			// Force le prochain affichage à requêter l'API Brevo plutôt que
			// d'afficher le cache (sinon le compteur ne reflète pas l'envoi).
			if sent > 0 {
				InvalidateBrevoCache()
			}

			c.Redirect(http.StatusFound, fmt.Sprintf("/messages?sent=%d&failed=%d", sent, failed))
			return
		}
	}

	// Feedback d'envoi (PRG via query).
	if v, err := strconv.Atoi(c.Query("sent")); err == nil {
		data.SendSuccess = v
	}
	if v, err := strconv.Atoi(c.Query("failed")); err == nil {
		data.SendFailed = v
	}
	if c.Query("nrcpt") == "1" {
		data.SendNoRcpt = true
	}

	t, err := loadTemplates("base.html", "design.html", "messages.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- /vendor/view/:id ----

type VendorViewData struct {
	PageData
	Vendor   model.Vendor
	Catalogs []model.Catalog
}

func (h *PagesHandler) VendorViewPage(c *gin.Context) {
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

	var vendor model.Vendor
	if err := h.db.First(&vendor, id).Error; err != nil {
		c.String(http.StatusNotFound, "producteur introuvable")
		return
	}

	var catalogs []model.Catalog
	h.db.Where("vendor_id = ? AND group_id = ?", id, pd.Group.ID).Find(&catalogs)

	data := VendorViewData{PageData: pd, Vendor: vendor, Catalogs: catalogs}
	data.Title = vendor.Name

	t, err := loadTemplates("base.html", "design.html", "vendor_view.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- helpers ----

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func paginateInts(total, page, perPage int) (pages []int) {
	n := total / perPage
	if total%perPage != 0 {
		n++
	}
	start := page - 2
	if start < 1 {
		start = 1
	}
	end := start + 4
	if end > n {
		end = n
	}
	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}
	return
}

// ensure time package used
var _ = time.Now
