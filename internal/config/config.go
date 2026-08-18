package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// Server
	Port  string `yaml:"port"`
	Debug bool   `yaml:"debug"`

	// Database
	DBHost     string `yaml:"db_host"`
	DBPort     string `yaml:"db_port"`
	DBUser     string `yaml:"db_user"`
	DBPassword string `yaml:"db_password"`
	DBName     string `yaml:"db_name"`

	// Auth
	JWTSecret      string `yaml:"jwt_secret"`
	JWTExpiryHours int    `yaml:"jwt_expiry_hours"`

	// Email
	SMTPHost     string `yaml:"smtp_host"`
	SMTPPort     string `yaml:"smtp_port"`
	SMTPUser     string `yaml:"smtp_user"`
	SMTPPassword string `yaml:"smtp_password"`
	DefaultEmail string `yaml:"default_email"`

	// Brevo API (pour récupérer le quota restant)
	BrevoAPIKey string `yaml:"brevo_api_key"`

	// App
	Host string `yaml:"host"`
	Key  string `yaml:"key"`

	// Responsable technique de l'installation (upsert au démarrage si email
	// renseigné).
	TechnicalManager TechnicalManagerConfig `yaml:"technical_manager"`

	// Ancien nom de la même notion, conservé en lecture seule : les
	// configurations déployées portent encore `superadmin`, et une clé ignorée
	// aurait fait démarrer l'application sans aucun responsable technique.
	// Fusionnée dans TechnicalManager au chargement.
	LegacySuperAdmin TechnicalManagerConfig `yaml:"superadmin"`

	// Notifications
	Notifications NotificationsConfig `yaml:"notifications"`

	// Messages (catégories de destinataires de la page /messages)
	Messages MessagesConfig `yaml:"messages"`

	// Interface (ce que l'application montre selon les droits)
	UI UIConfig `yaml:"ui"`
}

// UIConfig règle ce que voit un adhérent sans fonction dans le groupe.
type UIConfig struct {
	// VendorsTabForMembers rouvre l'onglet « Producteurs » à tout adhérent.
	//
	// Fermé par défaut : l'écran n'offre aucune action à qui n'administre ni le
	// groupe ni ses catalogues. Un groupe qui s'en sert comme annuaire de ses
	// producteurs le rouvre ici, sans qu'il faille reprendre le code — la
	// décision relève de l'usage de chaque groupe, pas de l'application.
	VendorsTabForMembers bool `yaml:"vendors_tab_for_members"`
}

// TechnicalManagerConfig décrit le compte du responsable technique, garanti au
// démarrage.
//
// Il remplace l'ancien super-administrateur, dont les pouvoirs tenaient à un bit
// en base : le rôle se lit maintenant dans la configuration, ce qui le rend
// unique pour toute l'installation, non attribuable depuis l'interface, et
// révocable en changeant une ligne de configuration plutôt qu'une ligne de
// table.
type TechnicalManagerConfig struct {
	Email     string `yaml:"email"`
	Password  string `yaml:"password"`
	FirstName string `yaml:"first_name"`
	LastName  string `yaml:"last_name"`
}

// fillFrom complète les champs vides depuis une autre déclaration, sans jamais
// écraser ce qui est déjà posé : le nom courant prime sur l'ancien.
func (t *TechnicalManagerConfig) fillFrom(other TechnicalManagerConfig) {
	if t.Email == "" {
		t.Email = other.Email
	}
	if t.Password == "" {
		t.Password = other.Password
	}
	if t.FirstName == "" {
		t.FirstName = other.FirstName
	}
	if t.LastName == "" {
		t.LastName = other.LastName
	}
}

// IsTechnicalManager désigne le responsable technique par son adresse. C'est le
// seul test : le rôle n'est inscrit nulle part en base, si bien qu'il suffit de
// changer la configuration pour le transférer, et qu'aucune écriture en base ne
// peut se l'octroyer.
func (c *Config) IsTechnicalManager(email string) bool {
	want := strings.TrimSpace(c.TechnicalManager.Email)
	if want == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(email), want)
}

type NotificationsConfig struct {
	// Enabled active globalement l'envoi des notifications email par le cron
	// (ouverture/fermeture commandes, rappels distrib). Défaut: true.
	// Si false, RecipientCategory n'est pas validé au boot.
	Enabled bool `yaml:"enabled"`

	// RecipientCategory : nom d'une catégorie de messages.recipient_categories.
	// Seuls les users matchant le pattern de cette catégorie reçoivent les
	// notifications d'ouverture/fermeture de commandes.
	// Sémantique : pure correspondance par pattern (pas d'exclusivité mutuelle).
	// Obligatoire si Enabled — boot fatal si vide ou si le nom ne correspond à aucune catégorie.
	RecipientCategory string `yaml:"recipient_category"`
}

// Load charge la configuration depuis (par ordre de priorité) :
//  1. le fichier YAML indiqué par CONFIG_FILE (défaut: config.yaml)
//  2. les variables d'environnement (fallback, utile pour les secrets Kube)
//  3. les valeurs par défaut
func Load() (*Config, error) {
	// Charge .env si présent (dev local), ignoré en prod
	_ = godotenv.Load()

	cfg := defaults()

	// Chargement YAML
	cfgFile := getEnv("CONFIG_FILE", "config.yaml")
	if err := loadYAML(cfgFile, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config file %q: %w", cfgFile, err)
	}

	// L'ancienne clé `superadmin` vaut pour `technical_manager` tant qu'une
	// configuration ne porte que la première. Champ par champ, et non bloc
	// entier : une configuration de transition peut ne renseigner que l'adresse
	// sous le nouveau nom et laisser le mot de passe sous l'ancien.
	cfg.TechnicalManager.fillFrom(cfg.LegacySuperAdmin)

	// Surcharge par variables d'environnement (secrets Kube, CI)
	overrideFromEnv(cfg)

	// Validation des champs obligatoires
	if cfg.DBPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD / db_password is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET / jwt_secret is required")
	}
	if cfg.Key == "" {
		return nil, fmt.Errorf("APP_KEY / key is required")
	}

	// Si aucune catégorie de destinataires n'est configurée, on en pose 3 par défaut
	// (cohérentes avec l'historique de l'app : réguliers / occasionnels / inactifs).
	if len(cfg.Messages.RecipientCategories) == 0 {
		cfg.Messages.RecipientCategories = defaultRecipientCategories()
	}

	// Compile les conditions de chaque catégorie. Les catégories invalides
	// sont ignorées avec un log.
	parsed := cfg.Messages.RecipientCategories[:0]
	for _, cat := range cfg.Messages.RecipientCategories {
		if err := cat.CompileConditions(); err != nil {
			log.Printf("warning: catégorie destinataire %q ignorée: %v", cat.Name, err)
			continue
		}
		parsed = append(parsed, cat)
	}
	cfg.Messages.RecipientCategories = parsed

	// Valide les références dans `includes` : chaque nom doit pointer vers
	// une catégorie existante. Sinon warning au log et la référence est
	// silencieusement ignorée à l'évaluation.
	known := make(map[string]bool, len(cfg.Messages.RecipientCategories))
	for _, cat := range cfg.Messages.RecipientCategories {
		known[cat.Name] = true
	}
	for _, cat := range cfg.Messages.RecipientCategories {
		for _, inc := range cat.Includes {
			if !known[inc] {
				log.Printf("warning: catégorie %q inclut %q qui n'existe pas — inclusion ignorée", cat.Name, inc)
			}
		}
	}

	// Si les notifications sont activées, recipient_category est obligatoire
	// et doit pointer vers une messages.recipient_categories existante.
	if cfg.Notifications.Enabled {
		name := strings.TrimSpace(cfg.Notifications.RecipientCategory)
		if name == "" {
			return nil, fmt.Errorf("notifications.recipient_category is required when notifications.enabled (nom d'une messages.recipient_categories)")
		}
		found := false
		for _, cat := range cfg.Messages.RecipientCategories {
			if cat.Name == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("notifications.recipient_category %q ne correspond à aucune messages.recipient_categories", name)
		}
	} else {
		log.Printf("[CONFIG] notifications désactivées (notifications.enabled=false)")
	}

	return cfg, nil
}

func defaultRecipientCategories() []RecipientCategory {
	return []RecipientCategory{
		{Name: "Membres réguliers", Pattern: ">= 3 commandes / 3m"},
		{Name: "Membres occasionnels", Pattern: ">= 1 commande / 6m"},
		{Name: "Membres inactifs", Pattern: "< 1 commande / 6m"},
	}
}

func defaults() *Config {
	return &Config{
		Port:           "8080",
		Debug:          false,
		DBHost:         "localhost",
		DBPort:         "3306",
		DBUser:         "alterconso",
		DBName:         "alterconso",
		JWTExpiryHours: 24 * 7,
		SMTPPort:       "587",
		DefaultEmail:   "noreply@alterconso.fr",
		Host:           "localhost",
		Notifications:  NotificationsConfig{Enabled: true},
	}
}

func loadYAML(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return yaml.NewDecoder(f).Decode(cfg)
}

// overrideFromEnv surcharge les champs du config avec les variables d'env si définies.
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("DEBUG"); v != "" {
		cfg.Debug = v == "true"
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.DBPort = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("APP_KEY"); v != "" {
		cfg.Key = v
	}
	if v := os.Getenv("SMTP_HOST"); v != "" {
		cfg.SMTPHost = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		cfg.SMTPPort = v
	}
	if v := os.Getenv("SMTP_USER"); v != "" {
		cfg.SMTPUser = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		cfg.SMTPPassword = v
	}
	if v := os.Getenv("DEFAULT_EMAIL"); v != "" {
		cfg.DefaultEmail = v
	}
	if v := os.Getenv("BREVO_API_KEY"); v != "" {
		cfg.BrevoAPIKey = v
	}
	if v := os.Getenv("HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("TECHNICAL_MANAGER_EMAIL"); v != "" {
		cfg.TechnicalManager.Email = v
	}
	if v := os.Getenv("TECHNICAL_MANAGER_PASSWORD"); v != "" {
		cfg.TechnicalManager.Password = v
	}
	// Anciens noms, encore posés par les déploiements en place.
	if v := os.Getenv("SUPERADMIN_EMAIL"); v != "" {
		cfg.TechnicalManager.Email = v
	}
	if v := os.Getenv("SUPERADMIN_PASSWORD"); v != "" {
		cfg.TechnicalManager.Password = v
	}
	if v := os.Getenv("NOTIFICATIONS_ENABLED"); v != "" {
		cfg.Notifications.Enabled = v == "true"
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
