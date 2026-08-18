package db

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// AppliedMigration trace les corrections de donnees deja passees.
//
// Les migrations de schema sont idempotentes par nature — AutoMigrate ne
// recree pas une colonne existante — mais une correction de donnees ne l'est
// pas toujours : celle qui rend un droit a quelques comptes le rendrait aussi,
// au demarrage suivant, a ceux a qui on vient de le retirer. D'ou cette trace.
type AppliedMigration struct {
	Name      string `gorm:"primaryKey;size:128"`
	AppliedAt time.Time
}

func (AppliedMigration) TableName() string { return "applied_migrations" }

// runOnce execute une correction de donnees si elle ne l'a jamais ete, et
// l'enregistre. La trace n'est posee qu'en cas de succes : une correction
// interrompue sera retentee au prochain demarrage plutot que consideree comme
// faite.
func runOnce(gdb *gorm.DB, name string, fn func(*gorm.DB) error) error {
	if err := gdb.AutoMigrate(&AppliedMigration{}); err != nil {
		return err
	}

	var n int64
	if err := gdb.Model(&AppliedMigration{}).Where("name = ?", name).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	if err := fn(gdb); err != nil {
		return err
	}

	log.Printf("[MIGRATION] %s appliquee", name)
	return gdb.Create(&AppliedMigration{Name: name, AppliedAt: time.Now()}).Error
}
