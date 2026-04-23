package db

import (
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Silent
	if cfg.Debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		// Désactive la création des contraintes FK lors du AutoMigrate.
		// Évite les erreurs de dépendances circulaires (ex: Group ↔ Place).
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Migrate crée ou met à jour les tables à partir des modèles Go.
// DisableForeignKeyConstraintWhenMigrating étant activé, l'ordre n'a pas d'importance.
// GORM ne supprime jamais de colonnes existantes — safe en production.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Vendor{},
		&model.Group{},
		&model.Place{},
		&model.UserGroup{},
		&model.Catalog{},
		&model.Product{},
		&model.MultiDistrib{},
		&model.Distribution{},
		&model.Subscription{},
		&model.Basket{},
		&model.UserOrder{},
		&model.Operation{},
		&model.Message{},
		&model.Membership{},
		&model.WaitingList{},
		&model.Volunteer{},
		&model.VolunteerRole{},
		&model.NotificationSent{},
		&model.PasswordResetToken{},
	)
}
