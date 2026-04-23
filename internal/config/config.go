package config

import (
	"fmt"
	"os"
	"time"

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

	// App
	Host string `yaml:"host"`
	Key  string `yaml:"key"`

	// Notifications
	Notifications NotificationsConfig `yaml:"notifications"`
}

type NotificationsConfig struct {
	// Durée maximale d'inactivité au-delà de laquelle un utilisateur
	// ne reçoit plus les notifications de commande.
	// Format Go duration : "720h" = 30 jours, "2160h" = 90 jours (3 mois).
	InactivityThreshold time.Duration `yaml:"inactivity_threshold"`
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

	return cfg, nil
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
		Notifications: NotificationsConfig{
			InactivityThreshold: 90 * 24 * time.Hour, // 3 mois par défaut
		},
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
	if v := os.Getenv("HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("NOTIF_INACTIVITY_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Notifications.InactivityThreshold = d
		}
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
