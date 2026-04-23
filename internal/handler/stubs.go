package handler

import "gorm.io/gorm"

// OrderHandler est défini ici, son implémentation est dans order.go
type OrderHandler struct{ db *gorm.DB }

func NewOrderHandler(db *gorm.DB) *OrderHandler { return &OrderHandler{db: db} }
