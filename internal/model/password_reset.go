package model

import "time"

// PasswordResetToken stocke les tokens de réinitialisation de mot de passe.
// Un token est valable 1 heure et est supprimé après utilisation.
type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	UserID    uint      `gorm:"not null;index"`
	Token     string    `gorm:"size:64;uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

func (p *PasswordResetToken) TableName() string { return "password_reset_tokens" }

func (p *PasswordResetToken) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}
