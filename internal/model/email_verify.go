package model

import "time"

// EmailVerifyToken : token d'activation envoyé par email à l'inscription.
// Valable 24h, supprimé après utilisation.
type EmailVerifyToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	UserID    uint      `gorm:"not null;index"`
	Token     string    `gorm:"size:64;uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

func (e *EmailVerifyToken) TableName() string { return "email_verify_tokens" }

func (e *EmailVerifyToken) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}
