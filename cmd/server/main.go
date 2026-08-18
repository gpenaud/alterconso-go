// Package main est le point d'entrée du serveur Alterconso.
//
//	@title          Alterconso API
//	@version        0.1.0
//	@description    API de gestion de groupements d'achat AMAP/CSA.
//	@contact.name   gpenaud
//	@contact.url    https://github.com/gpenaud/alterconso-go
//	@license.name   AGPL-3.0
//
//	@host     localhost:8080
//	@BasePath /api
//	@schemes  http https
//
//	@securityDefinitions.apikey BearerAuth
//	@in                         header
//	@name                       Authorization
//	@description                Format: "Bearer <token>"
package main

import (
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/gpenaud/alterconso/docs"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/db"
	"github.com/gpenaud/alterconso/internal/handler"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/service"
	"github.com/gpenaud/alterconso/pkg/mailer"

	"github.com/gin-gonic/gin"
)

// runRehash exécute le batch d'enrobage bcrypt des mots de passe MD5 legacy,
// puis quitte. Réutilise exactement la même config/DB que le serveur, donc
// lançable depuis l'image distroless via `/app/alterconso rehash` (kubectl
// exec sur le pod, ou Job k8s args:["rehash"]).
func runRehash(args []string) {
	fs := flag.NewFlagSet("rehash", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "ne rien écrire, juste compter les lignes legacy")
	batchSize := fs.Int("batch", 200, "taille des lots")
	_ = fs.Parse(args)

	// Le batch n'envoie aucune notification : on neutralise la validation du
	// bloc notifications (recipient_category) sans rapport avec le rehash, pour
	// rester auto-suffisant quelle que soit la config de l'image/du Job.
	_ = os.Setenv("NOTIFICATIONS_ENABLED", "false")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if _, _, _, err := db.RehashLegacyPasswords(database, *dryRun, *batchSize); err != nil {
		log.Fatalf("rehash: %v", err)
	}
}

func main() {
	// Sous-commandes one-shot (maintenance). Défaut sans argument = serveur.
	if len(os.Args) > 1 && os.Args[1] == "rehash" {
		runRehash(os.Args[2:])
		return
	}

	// Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Database
	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.Migrate(database); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	// Avant tout le reste : les gardes de routes lisent les droits convertis,
	// et un demarrage sur l'ancien modele ouvrirait la base a d'anciens
	// porteurs du role technique de groupe.
	if err := db.MigrateRightsModel(database); err != nil {
		log.Printf("warning: MigrateRightsModel: %v", err)
	}
	// Rattrapage ponctuel de la conversion ci-dessus : elle a retire les
	// membres et les catalogues a des gens qui s'en servaient.
	if err := db.RestoreDelegatedMembersAndCatalogs(database); err != nil {
		log.Printf("warning: RestoreDelegatedMembersAndCatalogs: %v", err)
	}
	// Repare l'elargissement errone de la correction ci-dessus.
	if err := db.RevertOverreachingMembersAndCatalogs(database); err != nil {
		log.Printf("warning: RevertOverreachingMembersAndCatalogs: %v", err)
	}
	if err := db.EnsureTechnicalManager(database, cfg); err != nil {
		log.Printf("warning: EnsureTechnicalManager: %v", err)
	}
	if err := db.SeedTxpCategories(database); err != nil {
		log.Printf("warning: SeedTxpCategories: %v", err)
	}
	if err := db.EnsureLegalRepAdmins(database); err != nil {
		log.Printf("warning: EnsureLegalRepAdmins: %v", err)
	}
	if err := db.BackfillVerifiedUsers(database); err != nil {
		log.Printf("warning: BackfillVerifiedUsers: %v", err)
	}

	// Subcommand "rights-report" : imprime les droits en vigueur, puis quitte.
	// Lecture seule — de quoi verifier ce qu'une correction a laisse.
	if len(os.Args) > 1 && os.Args[1] == "rights-report" {
		runRightsReport(database)
		return
	}

	// Subcommand "migrate" : exécute db.Migrate() puis quitte.
	// Utile pour préparer un import (création des tables GORM avant transformation Haxe).
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		log.Println("[MIGRATE] schema GORM appliqué.")
		return
	}

	// Subcommand "categorize-products" : auto-classifie les produits sans
	// taxonomie en se basant sur des mots-clés du nom du produit. Idempotent.
	if len(os.Args) > 1 && os.Args[1] == "categorize-products" {
		n, err := db.AutoCategorizeProducts(database)
		if err != nil {
			log.Fatalf("[CATEGORIZE] %v", err)
		}
		log.Printf("[CATEGORIZE] %d produit(s) classé(s).", n)
		return
	}

	m := mailer.New(cfg)
	cronSvc := service.NewCronService(database, m, cfg)

	// Subcommand "cron" : exécute les notifications une fois et quitte.
	// Utilisé par le Kubernetes CronJob.
	//   ./alterconso cron            → envoi réel
	//   ./alterconso cron --dry-run  → affiche destinataires + SQL, n'envoie rien
	if len(os.Args) > 1 && os.Args[1] == "cron" {
		dryRun := len(os.Args) > 2 && os.Args[2] == "--dry-run"
		if dryRun {
			log.Println("[CRON] dry-run mode — aucun email ne sera envoyé")
			cronSvc.SetDryRun(true)
		}
		log.Println("[CRON] running notifications...")
		cronSvc.RunAll()
		log.Println("[CRON] done.")
		return
	}

	// Mode serveur : goroutine de secours (désactivable si CronJob Kube actif)
	cronSvc.Start(time.Hour)

	// Router
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Middlewares globaux
	r.Use(middleware.CORS(cfg))

	// Routes
	handler.Register(r, database, cfg)

	log.Printf("alterconso listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
