package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
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
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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

// ---- POST /member/delete/:id ----

func (h *PagesHandler) MemberDelete(c *gin.Context) {
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
	h.db.Where("user_id = ? AND group_id = ?", id, pd.Group.ID).Delete(&model.UserGroup{})
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
	if pd.User == nil || pd.Group == nil || !pd.IsGroupManager {
		c.String(http.StatusForbidden, "accès refusé")
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
	SentMessages []MessageView
}

type MessageView struct {
	ID      uint
	Title   string
	Date    string
	Body    string
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

	if c.Request.Method == http.MethodPost {
		subject := strings.TrimSpace(c.PostForm("subject"))
		body := strings.TrimSpace(c.PostForm("body"))
		if subject != "" && body != "" {
			msg := model.Message{
				SenderID: pd.User.ID,
				GroupID:  pd.Group.ID,
				Subject:  subject,
				Body:     body,
			}
			h.db.Create(&msg)
			c.Redirect(http.StatusFound, "/messages")
			return
		}
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
