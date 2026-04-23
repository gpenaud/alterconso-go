package model

import "time"

// NotificationSent permet d'éviter les doublons d'envoi d'emails.
// L'index unique (multi_distrib_id, type) garantit qu'on n'envoie qu'une fois
// par distribution et par type de notification.
type NotificationSent struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	MultiDistribID uint      `gorm:"uniqueIndex:idx_notif_type;not null"`
	Type           string    `gorm:"size:32;uniqueIndex:idx_notif_type;not null"`
	SentAt         time.Time `gorm:"not null"`
}

func (n *NotificationSent) TableName() string { return "notifications_sent" }

const (
	NotifTypeOrderOpen    = "order_open"    // ouverture des commandes
	NotifTypeOrderClose24 = "order_close_24h" // 24h avant fermeture
)
