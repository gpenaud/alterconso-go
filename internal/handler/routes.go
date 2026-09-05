package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// Register enregistre toutes les routes de l'application.
func Register(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Le responsable technique se reconnaît à son adresse : on la recopie ici,
	// une fois, plutôt que de traîner la configuration jusqu'aux fonctions de
	// contrôle d'accès qui n'en ont aucun autre usage.
	SetTechnicalManager(cfg.TechnicalManager.Email)

	auth := middleware.Auth(cfg)
	pageAuth := middleware.PageAuth(cfg)

	// ---- Probes k8s ----
	// liveness  → /livez   : process vivant ? (zéro dépendance, jamais 503
	//                        sauf si Go est mort)
	// readiness → /healthz : DB joignable ? (timeout 1s, 503 sinon)
	r.GET("/livez", Liveness)
	r.GET("/healthz", Readiness(db))

	// ---- Static assets (original www/) ----
	// Sert les variantes pré-compressées (.br / .gz) générées au build Docker
	// quand le client les annonce dans Accept-Encoding ; sinon sert le fichier
	// original (cas des images binaires non pré-compressées).
	r.GET("/css/*filepath", StaticPrecompressed("/css", "www/css"))
	r.GET("/js/*filepath", StaticPrecompressed("/js", "www/js"))
	r.GET("/font/*filepath", StaticPrecompressed("/font", "www/font"))
	r.GET("/img/*filepath", StaticPrecompressed("/img", "www/img"))
	r.GET("/locales/*filepath", StaticPrecompressed("/locales", "www/locales"))
	fileH := NewFileHandler(db, cfg)
	r.GET("/file/:sign", fileH.ServeFile)

	// ---- SPA React (mountée à la racine) ----
	// /assets/* sert les bundles Vite (index-XXX.js / .css) directement depuis
	// frontend/dist/assets. Les routes SPA (/login, /groups/..., /shop/:id,
	// /profile) sont servies par index.html via NoRoute plus bas — React
	// Router prend le relais côté client. L'auth est portée par le cookie JWT
	// "token" déjà partagé avec les pages Go ; pas de middleware ici, les
	// appels API sous /api restent protégés par middleware.Auth.
	r.Static("/assets", "frontend/dist/assets")
	r.StaticFile("/favicon.svg", "frontend/dist/favicon.svg")

	// ---- Frontend pages (original Haxe UI) ----
	pagesH := NewPagesHandler(db, cfg)

	// Autorisation centralisée (fail-closed) pour les clusters admin.
	// La logique de droits reste celle du modèle existant (model.Right +
	// UserGroup.HasRight/IsGroupManager) ; ici on la DÉCLARE par route.
	// Les routes membre/publiques ne portent PAS ces middlewares.
	// Les contrôles fins par objet (catalogue/propriété) restent dans les
	// handlers en défense en profondeur.
	reqManager := pagesH.RequireGroupRight() // GroupAdmin requis
	// Pas de reqMessages : /messages est désormais ouverte à tout membre, le
	// droit Messages n'élargit plus l'accès mais le périmètre des
	// destinataires (cf. buildScopedRecipients).
	reqCatalog := pagesH.RequireGroupRight(model.RightCatalogAdmin) // gestionnaire ou CatalogAdmin
	// L'édition directe des tables n'appartient qu'au responsable de groupe et
	// au responsable technique : aucune délégation ne l'ouvre.
	reqDBAdmin := pagesH.RequireGroupRight()
	// Les deux délégations qui remplacent les anciens « droits administrateur » :
	// le calendrier d'un côté, les réglages du groupe de l'autre.
	reqDistributions := pagesH.RequireGroupRight(model.RightDistributions)
	reqParameters := pagesH.RequireGroupRight(model.RightParameters)
	reqMembership := pagesH.RequireGroupRight(model.RightMembership) // gestionnaire ou Membership
	// Plus étroit que reqManager : l'attribution des droits reste au
	// responsable de groupe et au responsable technique.
	reqRights := pagesH.RequireRightsManagement()

	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/home") })
	r.GET("/user/login", pagesH.LoginPage)
	r.GET("/user/logout", pagesH.Logout)
	r.GET("/user/return", pageAuth, pagesH.ImpersonateReturn)
	r.GET("/user/choose", pageAuth, pagesH.ChoosePage)
	r.GET("/home", pageAuth, pagesH.HomePage)
	// Fragment des periodes suivantes, pour le defilement continu de l accueil.
	r.GET("/home/more", pageAuth, pagesH.HomeMoreFragment)
	r.GET("/contract/view/:id", pageAuth, pagesH.ContractViewPage)
	r.GET("/account", pageAuth, pagesH.AccountPage)
	r.GET("/apropos", pageAuth, pagesH.AboutPage)
	r.GET("/account/edit", pageAuth, pagesH.AccountEditPage)
	r.POST("/account/update", pageAuth, pagesH.AccountUpdate)
	r.GET("/account/quit", pageAuth, pagesH.AccountQuitPage)
	// Le départ effectif : en POST, pour qu'un simple chargement d'adresse ne
	// puisse pas retirer un adhérent de son groupe.
	r.POST("/account/quit", pageAuth, pagesH.AccountQuitPage)
	r.GET("/member", pageAuth, pagesH.MemberPage)
	r.GET("/member/more", pageAuth, pagesH.MemberMoreFragment)
	r.GET("/distribution", pageAuth, pagesH.DistributionPage)
	r.GET("/amap", pageAuth, pagesH.AmapPage)
	r.GET("/amapadmin", pageAuth, reqParameters, pagesH.AmapAdminPage)
	r.GET("/amapadmin/edit", pageAuth, reqParameters, pagesH.AmapAdminEditPage)
	r.POST("/amapadmin/update", pageAuth, reqParameters, pagesH.AmapAdminUpdate)
	r.POST("/amapadmin/logo", pageAuth, reqParameters, pagesH.AmapAdminLogoUpload)
	r.GET("/amapadmin/logo/delete", pageAuth, reqParameters, pagesH.AmapAdminLogoDelete)
	r.GET("/amapadmin/rights", pageAuth, reqRights, pagesH.AmapAdminRightsPage)
	r.GET("/amapadmin/rights/add", pageAuth, reqRights, pagesH.AmapAdminRightsAddPage)
	r.POST("/amapadmin/rights/add", pageAuth, reqRights, pagesH.AmapAdminRightsAddPage)
	r.GET("/amapadmin/rights/edit/:userId", pageAuth, reqRights, pagesH.AmapAdminRightsEditPage)
	r.POST("/amapadmin/rights/edit/:userId", pageAuth, reqRights, pagesH.AmapAdminRightsEditPage)
	r.GET("/amapadmin/vatRates", pageAuth, reqParameters, pagesH.AmapAdminVatRatesPage)
	r.POST("/amapadmin/vatRates", pageAuth, reqParameters, pagesH.AmapAdminVatRatesUpdate)
	r.GET("/amapadmin/volunteers", pageAuth, reqParameters, pagesH.AmapAdminVolunteersPage)
	// Les postes de bénévole : l'écran les listait et proposait ces trois
	// liens, qu'aucune route ne servait.
	r.GET("/amapadmin/volunteers/new", pageAuth, reqParameters, pagesH.AmapAdminVolunteerNewPage)
	r.POST("/amapadmin/volunteers/new", pageAuth, reqParameters, pagesH.AmapAdminVolunteerNewPage)
	r.GET("/amapadmin/volunteers/edit/:id", pageAuth, reqParameters, pagesH.AmapAdminVolunteerEditPage)
	r.POST("/amapadmin/volunteers/edit/:id", pageAuth, reqParameters, pagesH.AmapAdminVolunteerEditPage)
	// En POST : supprimer un poste le retire des distributions qui l'avaient
	// retenu, et cela ne doit pas tenir au chargement d'une adresse.
	r.POST("/amapadmin/volunteers/delete/:id", pageAuth, reqParameters, pagesH.AmapAdminVolunteerDelete)
	r.GET("/amapadmin/membership", pageAuth, reqParameters, pagesH.AmapAdminMembershipPage)
	r.POST("/amapadmin/membership", pageAuth, reqParameters, pagesH.AmapAdminMembershipUpdate)
	r.GET("/amapadmin/currency", pageAuth, reqParameters, pagesH.AmapAdminCurrencyPage)
	r.POST("/amapadmin/currency", pageAuth, reqParameters, pagesH.AmapAdminCurrencyUpdate)
	r.GET("/amapadmin/documents", pageAuth, reqParameters, pagesH.AmapAdminDocumentsPage)
	r.POST("/amapadmin/documents", pageAuth, reqParameters, pagesH.AmapAdminDocumentsUpload)
	r.GET("/amapadmin/documents/delete/:id", pageAuth, reqParameters, pagesH.AmapAdminDocumentsDelete)

	// Espace d'administration : le point d'entree unique vers les ecrans de
	// gestion, depuis que la barre de navigation a disparu. Pas de middleware
	// de droit : le handler compose l ecran selon ce que l on detient, et
	// renvoie a l accueil s il n a rien a montrer.
	r.GET("/admin", pageAuth, pagesH.AdminHomePage)

	// ---- Admin base de données (droit DatabaseAdmin) ----
	r.GET("/admin/db", pageAuth, reqDBAdmin, pagesH.AdminDBIndex)
	r.GET("/admin/db/:slug", pageAuth, reqDBAdmin, pagesH.AdminDBList)
	r.GET("/admin/db/:slug/new", pageAuth, reqDBAdmin, pagesH.AdminDBNew)
	r.POST("/admin/db/:slug/new", pageAuth, reqDBAdmin, pagesH.AdminDBCreate)
	r.GET("/admin/db/:slug/edit/:id", pageAuth, reqDBAdmin, pagesH.AdminDBEdit)
	r.POST("/admin/db/:slug/edit/:id", pageAuth, reqDBAdmin, pagesH.AdminDBSave)
	r.POST("/admin/db/:slug/delete/:id", pageAuth, reqDBAdmin, pagesH.AdminDBDelete)

	// Group creation
	r.GET("/group/create/", pageAuth, pagesH.GroupCreatePage)
	r.POST("/group/create/", pageAuth, pagesH.GroupCreatePage)
	r.GET("/group/:id", pagesH.GroupPublicPage)
	r.GET("/contractAdmin", pageAuth, reqCatalog, pagesH.ContractAdminPage)

	// Member sub-pages (gestion de membres = gestionnaire).
	// /member/balance et /member/invoice restent membre (consultation perso) —
	// à valider produit, non verrouillés ici pour ne pas casser l'accès membre.
	r.GET("/member/view/:id", pageAuth, reqMembership, pagesH.MemberViewPage)
	r.GET("/member/loginAs/:id", pageAuth, reqMembership, pagesH.MemberLoginAs)
	r.GET("/member/payments/:id", pageAuth, reqMembership, pagesH.MemberPaymentsPage)
	r.GET("/member/balance", pageAuth, pagesH.MemberBalancePage)
	r.GET("/member/insert", pageAuth, reqMembership, pagesH.MemberInsertPage)
	r.POST("/member/insert", pageAuth, reqMembership, pagesH.MemberInsertPage)
	r.GET("/member/edit/:id", pageAuth, reqMembership, pagesH.MemberEditPage)
	r.POST("/member/edit/:id", pageAuth, reqMembership, pagesH.MemberEditPage)
	r.GET("/member/delete/:id", pageAuth, reqMembership, pagesH.MemberDelete)
	r.POST("/member/fullDelete/:id", pageAuth, reqManager, pagesH.MemberFullDelete)
	// Demandes d'adhésion au groupe, déposées à l'inscription. Elles
	// rétablissent la liste d'attente de l'application d'origine, dont la table
	// pointait le groupe ; la réécriture l'avait rattachée au catalogue, où
	// rien ne pouvait plus la remplir ni lui donner sens.
	//
	// reqMembership et non reqManager : « gestion des membres » est exactement
	// la délégation dont il s'agit, et le responsable de groupe la couvre.
	r.GET("/member/requests", pageAuth, reqMembership, pagesH.MemberRequestsPage)
	// La décision passe par POST : en GET, un préchargement de lien ferait
	// entrer quelqu'un dans le groupe sans que personne n'ait cliqué.
	r.POST("/member/requests/:id/:decision", pageAuth, reqMembership, pagesH.MemberRequestDecide)
	r.GET("/member/invoice/:multiDistribId", pageAuth, pagesH.MemberInvoicePage)
	r.POST("/member/membership/:id", pageAuth, reqMembership, pagesH.MembershipUpsert)

	// ContractAdmin sub-pages
	r.GET("/contractAdmin/ordersByDate/:date/:groupId", pageAuth, reqCatalog, pagesH.ContractAdminOrdersByDatePage)
	r.GET("/contractAdmin/vendorsByDate/:date/:groupId", pageAuth, reqCatalog, pagesH.ContractAdminVendorsByDatePage)
	r.GET("/contractAdmin/ordersByDate/:date/:groupId/csv", pageAuth, reqCatalog, pagesH.ContractAdminOrdersByDateCSV)
	r.GET("/contractAdmin/view/:id", pageAuth, reqCatalog, pagesH.CatalogAdminViewPage)
	r.GET("/contractAdmin/edit/:id", pageAuth, reqCatalog, pagesH.CatalogAdminEditPage)
	r.POST("/contractAdmin/edit/:id", pageAuth, reqCatalog, pagesH.CatalogAdminEditPage)
	r.GET("/contractAdmin/duplicate/:id", pageAuth, reqCatalog, pagesH.CatalogAdminDuplicatePage)
	r.POST("/contractAdmin/duplicate/:id", pageAuth, reqCatalog, pagesH.CatalogAdminDuplicatePage)
	r.GET("/contractAdmin/products/:id", pageAuth, reqCatalog, pagesH.CatalogAdminProductsPage)
	r.GET("/contractAdmin/products/:id/importcsv", pageAuth, reqCatalog, pagesH.CatalogAdminProductsImportCSV)
	r.POST("/contractAdmin/products/:id/importcsv", pageAuth, reqCatalog, pagesH.CatalogAdminProductsImportCSV)
	r.POST("/contractAdmin/products/:id/bulkAction", pageAuth, reqCatalog, pagesH.CatalogAdminProductsBulkAction)
	r.GET("/contractAdmin/products/:id/new", pageAuth, reqCatalog, pagesH.CatalogAdminProductNewPage)
	r.POST("/contractAdmin/products/:id/new", pageAuth, reqCatalog, pagesH.CatalogAdminProductNewPage)
	r.GET("/contractAdmin/products/:id/edit/:productId", pageAuth, reqCatalog, pagesH.CatalogAdminProductEditPage)
	r.POST("/contractAdmin/products/:id/edit/:productId", pageAuth, reqCatalog, pagesH.CatalogAdminProductEditPage)
	r.GET("/contractAdmin/products/:id/photo/:productId", pageAuth, reqCatalog, pagesH.CatalogAdminProductPhotoPage)
	r.POST("/contractAdmin/products/:id/photo/:productId", pageAuth, reqCatalog, pagesH.CatalogAdminProductPhotoPage)
	r.GET("/contractAdmin/products/:id/delete/:productId", pageAuth, reqCatalog, pagesH.CatalogAdminProductDeletePage)
	r.GET("/contractAdmin/distributions/:id", pageAuth, reqCatalog, pagesH.CatalogAdminDistributionsPage)
	r.POST("/contractAdmin/distributions/:id", pageAuth, reqCatalog, pagesH.CatalogAdminDistributionsPage)
	// Dates d'une distribution pour ce seul catalogue. reqCatalog et non
	// reqManager : c'est le producteur qui sait s'il peut encore accepter une
	// commande, et donc rouvrir la sienne.
	r.GET("/contractAdmin/distributions/:id/dates/:distribId", pageAuth, reqCatalog, pagesH.CatalogAdminDistributionDatesPage)
	r.POST("/contractAdmin/distributions/:id/dates/:distribId", pageAuth, reqCatalog, pagesH.CatalogAdminDistributionDatesPage)
	r.GET("/contractAdmin/orders/:id", pageAuth, reqCatalog, pagesH.CatalogAdminOrdersPage)
	r.GET("/contractAdmin/selectDistrib/:id", pageAuth, reqCatalog, pagesH.CatalogAdminSelectDistribPage)
	r.GET("/contractAdmin/memberOrder/:multiDistribId/:userId", pageAuth, reqCatalog, pagesH.MemberOrderPage)
	r.POST("/contractAdmin/memberOrder/:multiDistribId/:userId", pageAuth, reqCatalog, pagesH.MemberOrderPage)
	r.POST("/contractAdmin/updateOrders/:multiDistribId/:userId", pageAuth, reqCatalog, pagesH.UpdateMemberOrders)
	r.POST("/contractAdmin/addProduct/:multiDistribId/:userId", pageAuth, reqCatalog, pagesH.AddMemberProduct)
	r.POST("/contractAdmin/deleteOrder/:multiDistribId/:userId/:orderId", pageAuth, reqCatalog, pagesH.DeleteMemberOrder)
	r.GET("/contractAdmin/subscriptions/:id", pageAuth, reqCatalog, pagesH.CatalogAdminSubscriptionsPage)

	// Distribution admin
	r.GET("/distribution/editMd/:id", pageAuth, reqDistributions, pagesH.DistributionEditMdPage)
	r.POST("/distribution/editMd/:id", pageAuth, reqDistributions, pagesH.DistributionEditMdPage)
	r.GET("/distribution/deleteMd/:id", pageAuth, reqDistributions, pagesH.DistributionDeleteMdPage)
	r.GET("/distribution/insertMd", pageAuth, reqDistributions, pagesH.DistributionInsertMdPage)
	r.POST("/distribution/insertMd", pageAuth, reqDistributions, pagesH.DistributionInsertMdPage)
	// L'ancienne adresse de programmation mene desormais a l'ecran dedie, qui
	// liste, cree et modifie. Conservee en redirection : un signet ou un lien
	// garde ailleurs ne doit pas tomber sur une page morte.
	r.GET("/distribution/insertMdCycle", pageAuth, reqDistributions, func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/distribution/cycles/new")
	})
	// Les cycles et leur courrier d'ouverture. Sous la meme delegation que le
	// calendrier : ce courrier annonce l'ouverture des commandes, il appartient
	// a qui tient les distributions et non a qui ecrit aux membres.
	// Sas vers la boutique, porte par les courriers : il garantit la connexion
	// la ou /shop/:id, servie par la SPA, repond 200 sans session et n echoue
	// qu ensuite sur ses appels d API.
	r.GET("/distribution/order/:id", pageAuth, pagesH.DistributionOrderRedirect)
	r.GET("/distribution/cycles", pageAuth, reqDistributions, pagesH.DistributionCyclesPage)
	r.GET("/distribution/cycles/new", pageAuth, reqDistributions, pagesH.CycleNewPage)
	r.POST("/distribution/cycles/new", pageAuth, reqDistributions, pagesH.CycleNewPage)
	r.GET("/distribution/cycles/:id", pageAuth, reqDistributions, pagesH.CycleEditPage)
	r.POST("/distribution/cycles/:id", pageAuth, reqDistributions, pagesH.CycleEditPage)
	// En POST : la suppression detruit des journees de calendrier, et le
	// prechargement d un lien suffirait a la declencher.
	r.POST("/distribution/cycles/:id/delete", pageAuth, reqDistributions, pagesH.CycleDeleteAction)
	r.GET("/distribution/validate/:id", pageAuth, reqDistributions, pagesH.DistributionValidatePage)
	r.GET("/distribution/inviteFarmers/:id", pageAuth, reqDistributions, pagesH.DistributionInviteFarmersPage)
	// La page listait les producteurs sans permettre d'en ajouter ni d'en
	// retirer, alors que le bouton qui y mène le promet : elle reçoit son POST.
	r.POST("/distribution/inviteFarmers/:id", pageAuth, reqDistributions, pagesH.DistributionInviteFarmersPage)
	// POST, et non GET : ce retrait supprime la distribution d'un producteur.
	// En GET, un préchargement de lien suffisait à le déclencher.
	r.POST("/distribution/notAttend/:id", pageAuth, reqDistributions, pagesH.DistributionNotAttendPage)
	r.GET("/distribution/shift/:id", pageAuth, reqDistributions, pagesH.DistributionShiftPage)
	r.POST("/distribution/shift/:id", pageAuth, reqDistributions, pagesH.DistributionShiftPage)
	r.GET("/edit/:id", pageAuth, reqManager, pagesH.DistributionEditDatesPage)
	r.POST("/edit/:id", pageAuth, reqManager, pagesH.DistributionEditDatesPage)
	// Self-service membre (se désinscrire / s'inscrire à un créneau bénévole,
	// consulter les listes de distribution) : NON verrouillé volontairement.
	r.GET("/distribution/volunteers/:id/unregister", pageAuth, pagesH.VolunteerUnregisterPage)
	r.GET("/distribution/volunteersCalendar", pageAuth, pagesH.VolunteersCalendarPage)
	r.POST("/distribution/volunteersCalendar/join", pageAuth, pagesH.VolunteersCalendarJoin)
	r.POST("/distribution/volunteersCalendar/leave", pageAuth, pagesH.VolunteersCalendarLeave)
	r.GET("/distribution/list/:id", pageAuth, pagesH.DistributionListPage)
	r.GET("/distribution/listByDate/:date/:groupId", pageAuth, pagesH.DistributionListByDateConfigPage)
	r.GET("/distribution/listByDate/:date/:groupId/print", pageAuth, pagesH.DistributionListByDatePrintPage)
	r.GET("/distribution/volunteersSummary/:id", pageAuth, reqDistributions, pagesH.VolunteersSummaryPage)
	r.POST("/distribution/volunteersSummary/:id", pageAuth, reqDistributions, pagesH.VolunteersSummaryPage)
	r.GET("/distribution/roles/:id", pageAuth, reqDistributions, pagesH.DistribRolesPage)
	r.POST("/distribution/roles/:id", pageAuth, reqDistributions, pagesH.DistribRolesPage)
	r.GET("/distribution/volunteersParticipation", pageAuth, pagesH.VolunteersParticipationPage)

	// Vendor
	r.GET("/vendor/view/:id", pageAuth, pagesH.VendorViewPage)
	// Ecriture des fiches producteurs. Aucun middleware de droit ici : la
	// fiche n'appartient a aucun groupe, et c'est le handler qui verifie le
	// rattachement en plus du droit — un « reqManager » laisserait un
	// responsable reecrire la ferme du groupe voisin.
	r.GET("/vendor/insert", pageAuth, pagesH.VendorInsertPage)
	r.POST("/vendor/insert", pageAuth, pagesH.VendorInsertPage)
	r.GET("/vendor/edit/:id", pageAuth, pagesH.VendorEditPage)
	r.POST("/vendor/edit/:id", pageAuth, pagesH.VendorEditPage)
	// En POST : un lien preleve par un antivirus de messagerie ou preparé
	// par le navigateur effacerait la fiche sans que personne ait clique.
	r.POST("/vendor/delete/:id", pageAuth, pagesH.VendorDelete)

	// Messages
	// Ouverte à tout membre connecté : le handler restreint les destinataires
	// selon les droits (gestionnaire → tout le groupe ; responsable de
	// catalogue → ses clients ; adhérent → responsables et contacts
	// techniques). Le filtrage ne peut pas vivre dans un middleware, qui ne
	// sait qu'autoriser ou refuser la route entière.
	r.GET("/messages", pageAuth, pagesH.MessagesPage)
	r.POST("/messages", pageAuth, pagesH.MessagesPage)

	// Transactions (saisie de paiement = gestionnaire ou droit Membership,
	// cohérent avec le gate interne de InsertPaymentPage)
	r.GET("/transaction/insertPayment/:memberId", pageAuth, reqMembership, pagesH.InsertPaymentPage)
	r.POST("/transaction/insertPayment/:memberId", pageAuth, reqMembership, pagesH.InsertPaymentPage)

	// Auth
	r.GET("/user/forgottenPassword", pagesH.ForgotPasswordPage)
	r.POST("/user/forgottenPassword", pagesH.ForgotPasswordPage)
	r.GET("/user/register", pagesH.RegisterPage)
	r.POST("/user/register", pagesH.RegisterPage)
	r.GET("/user/verify", pagesH.VerifyEmailPage)
	r.GET("/user/completeProfile", pageAuth, pagesH.CompleteProfilePage)
	r.POST("/user/completeProfile", pageAuth, pagesH.CompleteProfilePage)
	r.GET("/user/definePassword", pagesH.DefinePasswordPage)
	r.POST("/user/definePassword", pagesH.DefinePasswordPage)
	r.GET("/user/resetPassword", pagesH.DefinePasswordPage)
	r.POST("/user/resetPassword", pagesH.DefinePasswordPage)

	// ---- API compatibilité frontend original ----
	compatH := NewCompatHandler(db, cfg)
	// Login public (pas de middleware auth).
	//
	// L'inscription ne figure plus ici. Elle y créait un compte sans confirmer
	// l'adresse et sans demande d'adhésion : depuis que rejoindre un groupe
	// passe par l'accord d'un gestionnaire, un compte né par cette voie
	// n'aurait plus aucun moyen d'aboutir quelque part. Aucun client ne
	// l'appelait — ni le frontend d'origine, ni la SPA. /user/register est le
	// seul chemin d'inscription.
	r.POST("/api/user/login", compatH.UserLogin)
	// Endpoints authentifiés via cookie ou Bearer
	apiCompat := r.Group("/api", auth)
	apiCompat.GET("/user/me", compatH.UserMe)
	apiCompat.GET("/user/getFromGroup/", compatH.UserGetFromGroup)
	apiCompat.GET("/order/catalogs/:multiDistribId", compatH.OrderCatalogs)
	apiCompat.GET("/order/get/:userId", compatH.OrderGet)
	apiCompat.POST("/order/update/:userId", compatH.OrderUpdate)
	// Session courante, pour que la SPA puisse afficher le bandeau
	// d'impersonation que les pages Go rendent depuis design.html.
	apiCompat.GET("/session", compatH.SessionInfo)
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
	orderH := NewOrderHandler(db, cfg)
	api.GET("/orders", orderH.GetForUser)
	api.POST("/orders", orderH.CreateOrUpdate)

	// Home + Account (JSON pour les pages React).
	api.GET("/home", pagesH.HomeJSON)
	api.GET("/account", pagesH.AccountJSON)

	// ---- Fallback SPA ----
	// Pour toute route non matchée par les pages Go ou l'API, on sert
	// frontend/dist/index.html si l'URL ressemble à une route SPA. Sinon 404.
	// Ça permet à React Router de gérer /shop/:id, /groups/:id/..., /login,
	// /profile sans préfixe.
	spaPrefixes := []string{"/login", "/profile", "/groups", "/shop/"}
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		for _, pref := range spaPrefixes {
			if p == pref || strings.HasPrefix(p, pref+"/") ||
				(strings.HasSuffix(pref, "/") && strings.HasPrefix(p, pref)) {
				c.File("frontend/dist/index.html")
				return
			}
		}
		c.Status(http.StatusNotFound)
	})
}
