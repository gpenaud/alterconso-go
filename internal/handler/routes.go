package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// Register enregistre toutes les routes de l'application.
func Register(r *gin.Engine, db *gorm.DB, cfg *config.Config) {

	auth := middleware.Auth(cfg)
	pageAuth := middleware.PageAuth(cfg)

	// ---- Probes ----
	r.GET("/livez", func(c *gin.Context) { c.JSON(200, gin.H{"status": "alive"}) })
	r.GET("/healthz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "error": "database unreachable"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ---- Static assets (original www/) ----
	r.Static("/css", "www/css")
	r.Static("/js", "www/js")
	r.Static("/font", "www/font")
	r.Static("/img", "www/img")
	fileH := NewFileHandler(db, cfg)
	r.GET("/file/:sign", fileH.ServeFile)

	// ---- Frontend pages (original Haxe UI) ----
	pagesH := NewPagesHandler(db, cfg)
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/home") })
	r.GET("/user/login", pagesH.LoginPage)
	r.GET("/user/logout", pagesH.Logout)
	r.GET("/user/choose", pageAuth, pagesH.ChoosePage)
	r.GET("/home", pageAuth, pagesH.HomePage)
	r.GET("/contract/view/:id", pageAuth, pagesH.ContractViewPage)
	r.GET("/shop/:multiDistribId", pageAuth, pagesH.ShopPage)
	r.GET("/account", pageAuth, pagesH.AccountPage)
	r.GET("/account/edit", pageAuth, pagesH.AccountEditPage)
	r.POST("/account/update", pageAuth, pagesH.AccountUpdate)
	r.GET("/account/quit", pageAuth, pagesH.AccountQuitPage)
	r.GET("/member", pageAuth, pagesH.MemberPage)
	r.GET("/distribution", pageAuth, pagesH.DistributionPage)
	r.GET("/amap", pageAuth, pagesH.AmapPage)
	r.GET("/amapadmin", pageAuth, pagesH.AmapAdminPage)
	r.POST("/amapadmin/update", pageAuth, pagesH.AmapAdminUpdate)
	r.GET("/amapadmin/rights", pageAuth, pagesH.AmapAdminRightsPage)
	r.GET("/amapadmin/vatRates", pageAuth, pagesH.AmapAdminVatRatesPage)
	r.POST("/amapadmin/vatRates", pageAuth, pagesH.AmapAdminVatRatesUpdate)
	r.GET("/amapadmin/volunteers", pageAuth, pagesH.AmapAdminVolunteersPage)
	r.GET("/amapadmin/membership", pageAuth, pagesH.AmapAdminMembershipPage)
	r.POST("/amapadmin/membership", pageAuth, pagesH.AmapAdminMembershipUpdate)
	r.GET("/amapadmin/currency", pageAuth, pagesH.AmapAdminCurrencyPage)
	r.POST("/amapadmin/currency", pageAuth, pagesH.AmapAdminCurrencyUpdate)
	r.GET("/amapadmin/documents", pageAuth, pagesH.AmapAdminDocumentsPage)

	// Group creation
	r.GET("/group/create/", pageAuth, pagesH.GroupCreatePage)
	r.POST("/group/create/", pageAuth, pagesH.GroupCreatePage)
	r.GET("/contractAdmin", pageAuth, pagesH.ContractAdminPage)

	// Member sub-pages
	r.GET("/member/view/:id", pageAuth, pagesH.MemberViewPage)
	r.GET("/member/payments/:id", pageAuth, pagesH.MemberPaymentsPage)
	r.GET("/member/balance", pageAuth, pagesH.MemberBalancePage)
	r.GET("/member/insert", pageAuth, pagesH.MemberInsertPage)
	r.POST("/member/insert", pageAuth, pagesH.MemberInsertPage)
	r.GET("/member/edit/:id", pageAuth, pagesH.MemberEditPage)
	r.POST("/member/edit/:id", pageAuth, pagesH.MemberEditPage)
	r.GET("/member/delete/:id", pageAuth, pagesH.MemberDelete)
	r.GET("/member/waiting", pageAuth, pagesH.MemberWaitingPage)
	r.GET("/member/invoice/:multiDistribId", pageAuth, pagesH.MemberInvoicePage)

	// ContractAdmin sub-pages
	r.GET("/contractAdmin/ordersByDate/:date/:groupId", pageAuth, pagesH.ContractAdminOrdersByDatePage)
	r.GET("/contractAdmin/vendorsByDate/:date/:groupId", pageAuth, pagesH.ContractAdminVendorsByDatePage)
	r.GET("/contractAdmin/ordersByDate/:date/:groupId/csv", pageAuth, pagesH.ContractAdminOrdersByDateCSV)
	r.GET("/contractAdmin/view/:id", pageAuth, pagesH.CatalogAdminViewPage)
	r.GET("/contractAdmin/edit/:id", pageAuth, pagesH.CatalogAdminEditPage)
	r.POST("/contractAdmin/edit/:id", pageAuth, pagesH.CatalogAdminEditPage)
	r.GET("/contractAdmin/duplicate/:id", pageAuth, pagesH.CatalogAdminDuplicatePage)
	r.POST("/contractAdmin/duplicate/:id", pageAuth, pagesH.CatalogAdminDuplicatePage)
	r.GET("/contractAdmin/products/:id", pageAuth, pagesH.CatalogAdminProductsPage)
	r.POST("/contractAdmin/products/:id/bulkAction", pageAuth, pagesH.CatalogAdminProductsBulkAction)
	r.GET("/contractAdmin/products/:id/edit/:productId", pageAuth, pagesH.CatalogAdminProductEditPage)
	r.POST("/contractAdmin/products/:id/edit/:productId", pageAuth, pagesH.CatalogAdminProductEditPage)
	r.GET("/contractAdmin/products/:id/photo/:productId", pageAuth, pagesH.CatalogAdminProductPhotoPage)
	r.POST("/contractAdmin/products/:id/photo/:productId", pageAuth, pagesH.CatalogAdminProductPhotoPage)
	r.GET("/contractAdmin/products/:id/delete/:productId", pageAuth, pagesH.CatalogAdminProductDeletePage)
	r.GET("/contractAdmin/distributions/:id", pageAuth, pagesH.CatalogAdminDistributionsPage)
	r.POST("/contractAdmin/distributions/:id", pageAuth, pagesH.CatalogAdminDistributionsPage)
	r.GET("/contractAdmin/orders/:id", pageAuth, pagesH.CatalogAdminOrdersPage)
	r.GET("/contractAdmin/subscriptions/:id", pageAuth, pagesH.CatalogAdminSubscriptionsPage)

	// Distribution admin
	r.GET("/distribution/editMd/:id", pageAuth, pagesH.DistributionEditMdPage)
	r.POST("/distribution/editMd/:id", pageAuth, pagesH.DistributionEditMdPage)
	r.GET("/distribution/deleteMd/:id", pageAuth, pagesH.DistributionDeleteMdPage)
	r.GET("/distribution/insertMd", pageAuth, pagesH.DistributionInsertMdPage)
	r.POST("/distribution/insertMd", pageAuth, pagesH.DistributionInsertMdPage)
	r.GET("/distribution/insertMdCycle", pageAuth, pagesH.DistributionInsertMdCyclePage)
	r.POST("/distribution/insertMdCycle", pageAuth, pagesH.DistributionInsertMdCyclePage)
	r.GET("/distribution/validate/:id", pageAuth, pagesH.DistributionValidatePage)
	r.GET("/distribution/inviteFarmers/:id", pageAuth, pagesH.DistributionInviteFarmersPage)
	r.GET("/distribution/notAttend/:id", pageAuth, pagesH.DistributionNotAttendPage)
	r.GET("/distribution/shift/:id", pageAuth, pagesH.DistributionShiftPage)
	r.POST("/distribution/shift/:id", pageAuth, pagesH.DistributionShiftPage)
	r.GET("/edit/:id", pageAuth, pagesH.DistributionEditDatesPage)
	r.POST("/edit/:id", pageAuth, pagesH.DistributionEditDatesPage)
	r.GET("/distribution/volunteers/:id/unregister", pageAuth, pagesH.VolunteerUnregisterPage)
	r.GET("/distribution/volunteersCalendar", pageAuth, pagesH.VolunteersCalendarPage)
	r.POST("/distribution/volunteersCalendar/join", pageAuth, pagesH.VolunteersCalendarJoin)
	r.POST("/distribution/volunteersCalendar/leave", pageAuth, pagesH.VolunteersCalendarLeave)
	r.GET("/distribution/list/:id", pageAuth, pagesH.DistributionListPage)
	r.GET("/distribution/listByDate/:date/:groupId", pageAuth, pagesH.DistributionListByDateConfigPage)
	r.GET("/distribution/listByDate/:date/:groupId/print", pageAuth, pagesH.DistributionListByDatePrintPage)
	r.GET("/distribution/volunteersSummary/:id", pageAuth, pagesH.VolunteersSummaryPage)
	r.POST("/distribution/volunteersSummary/:id", pageAuth, pagesH.VolunteersSummaryPage)
	r.GET("/distribution/roles/:id", pageAuth, pagesH.DistribRolesPage)
	r.POST("/distribution/roles/:id", pageAuth, pagesH.DistribRolesPage)
	r.GET("/distribution/volunteersParticipation", pageAuth, pagesH.VolunteersParticipationPage)

	// Vendor
	r.GET("/vendor/view/:id", pageAuth, pagesH.VendorViewPage)

	// Messages
	r.GET("/messages", pageAuth, pagesH.MessagesPage)
	r.POST("/messages", pageAuth, pagesH.MessagesPage)

	// Transactions
	r.GET("/transaction/insertPayment/:memberId", pageAuth, pagesH.InsertPaymentPage)
	r.POST("/transaction/insertPayment/:memberId", pageAuth, pagesH.InsertPaymentPage)

	// Auth
	r.GET("/user/forgottenPassword", pagesH.ForgotPasswordPage)
	r.POST("/user/forgottenPassword", pagesH.ForgotPasswordPage)
	r.GET("/user/definePassword", pagesH.DefinePasswordPage)
	r.POST("/user/definePassword", pagesH.DefinePasswordPage)
	r.GET("/user/resetPassword", pagesH.DefinePasswordPage)
	r.POST("/user/resetPassword", pagesH.DefinePasswordPage)

	// ---- API compatibilité frontend original ----
	compatH := NewCompatHandler(db, cfg)
	// Login / register public (pas de middleware auth)
	r.POST("/api/user/login", compatH.UserLogin)
	r.POST("/api/user/register", compatH.UserRegister)
	// Endpoints authentifiés via cookie ou Bearer
	apiCompat := r.Group("/api", auth)
	apiCompat.GET("/user/me", compatH.UserMe)
	apiCompat.GET("/user/getFromGroup/", compatH.UserGetFromGroup)
	apiCompat.GET("/order/catalogs/:multiDistribId", compatH.OrderCatalogs)
	apiCompat.GET("/order/get/:userId", compatH.OrderGet)
	apiCompat.POST("/order/update/:userId", compatH.OrderUpdate)
	apiCompat.GET("/product/get/", compatH.ProductGet)
	apiCompat.GET("/product/categories", compatH.ProductCategories)
	apiCompat.GET("/planning/:groupId", compatH.Planning)
	apiCompat.GET("/shop/init", compatH.ShopInit)
	apiCompat.GET("/shop/allProducts", compatH.ShopAllProducts)
	apiCompat.GET("/shop/categories", compatH.ShopCategories)
	apiCompat.POST("/shop/submit/:multiDistribId", compatH.ShopSubmit)

	// ---- Swagger UI ----
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ---- Auth (API moderne) ----
	authH := NewAuthHandler(db, cfg)
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/logout", auth, authH.Logout)

	api := r.Group("/api", auth)

	// Users
	userH := NewUserHandler(db, cfg)
	api.GET("/users/me", userH.Me)
	api.PUT("/users/me", userH.UpdateMe)
	api.GET("/users", userH.List)
	api.GET("/users/:id", userH.Get)
	api.PUT("/users/:id", userH.Update)

	// Groups — toutes les routes sous /api/groups/:id utilisent le même paramètre
	groupH := NewGroupHandler(db, cfg)
	api.GET("/groups", groupH.List)
	api.POST("/groups", groupH.Create)
	groups := api.Group("/groups/:id")
	{
		groups.GET("", groupH.Get)
		groups.PUT("", groupH.Update)

		// Sous-ressources du groupe
		vendorH := NewVendorHandler(db)
		groups.GET("/vendors", vendorH.List)
		groups.POST("/vendors", vendorH.Create)

		catalogH := NewCatalogHandler(db)
		groups.GET("/catalogs", catalogH.List)
		groups.POST("/catalogs", catalogH.Create)

		distribH := NewDistributionHandler(db)
		groups.GET("/distributions", distribH.List)

		memberH := NewMemberHandler(db)
		groups.GET("/members", memberH.List)
		groups.POST("/members", memberH.Add)
		groups.DELETE("/members/:userId", memberH.Remove)

		payH := NewPaymentHandler(db)
		groups.GET("/payment-types", payH.GetPaymentTypes)
		groups.POST("/payments", payH.CreatePayment)
		groups.GET("/operations", payH.GetOperations)

		finH := NewFinanceHandler(db)
		groups.GET("/balance", finH.GetBalance)
		groups.GET("/finances", finH.GetGroupFinances)
		groups.GET("/finances/:userId", finH.GetUserFinances)
	}

	// Ressources standalone
	vendorH := NewVendorHandler(db)
	api.PUT("/vendors/:id", vendorH.Update)

	catalogH := NewCatalogHandler(db)
	catalogs := api.Group("/catalogs/:id")
	{
		catalogs.GET("", catalogH.Get)
		catalogs.PUT("", catalogH.Update)

		subH := NewSubscriptionHandler(db)
		catalogs.GET("/subscriptions", subH.GetForCatalog)
		catalogs.POST("/subscriptions", subH.Subscribe)

		wlH := NewWaitingListHandler(db)
		catalogs.GET("/waiting-list", wlH.GetForCatalog)
		catalogs.POST("/waiting-list", wlH.Join)
		catalogs.DELETE("/waiting-list", wlH.Leave)
	}

	distribH := NewDistributionHandler(db)
	distrib := api.Group("/distributions/:id")
	{
		distrib.GET("", distribH.Get)

		volH := NewVolunteerHandler(db)
		distrib.GET("/volunteers", volH.GetForDistrib)
		distrib.POST("/volunteers", volH.Register)

		payH := NewPaymentHandler(db)
		distrib.POST("/validate", payH.ValidateDistribution)
	}

	// Subscriptions & Volunteers (ressources directes)
	subH := NewSubscriptionHandler(db)
	api.DELETE("/subscriptions/:id", subH.Unsubscribe)

	volH := NewVolunteerHandler(db)
	api.DELETE("/volunteers/:id", volH.Unregister)

	// Orders
	orderH := NewOrderHandler(db)
	api.GET("/orders", orderH.GetForUser)
	api.POST("/orders", orderH.CreateOrUpdate)
}
