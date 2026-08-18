package handler

import (
	"github.com/gpenaud/alterconso/internal/config"
	"gorm.io/gorm"
)

// OrderHandler est défini ici, son implémentation est dans order.go
type OrderHandler struct {
	db *gorm.DB
	// Le responsable technique se reconnait a une adresse posee en
	// configuration : sans elle, ce handler ne peut plus decider qui consulte
	// la commande d'un autre.
	cfg *config.Config
}

func NewOrderHandler(db *gorm.DB, cfg *config.Config) *OrderHandler {
	return &OrderHandler{db: db, cfg: cfg}
}
