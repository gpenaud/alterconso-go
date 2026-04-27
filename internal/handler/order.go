package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/internal/service"
)

// OrderResponse est la représentation JSON d'une commande envoyée au client.
type OrderResponse struct {
	ID           uint    `json:"id"`
	UserID       uint    `json:"userId"`
	UserName     string  `json:"userName"`
	User2ID      *uint   `json:"userId2,omitempty"`
	ProductID    uint    `json:"productId"`
	ProductName  string  `json:"productName"`
	ProductPrice float64 `json:"productPrice"`
	Quantity     float64 `json:"quantity"`
	FeesRate     float64 `json:"feesRate"`
	Fees         float64 `json:"fees"`
	SubTotal     float64 `json:"subTotal"`
	Total        float64 `json:"total"`
	Paid         bool    `json:"paid"`
	CatalogID    uint    `json:"catalogId"`
	CatalogName  string  `json:"catalogName"`
	CanModify    bool    `json:"canModify"`
}

func prepareOrder(o model.UserOrder) OrderResponse {
	subTotal := o.Quantity * o.ProductPrice
	fees := subTotal * (o.FeesRate / 100)
	return OrderResponse{
		ID:           o.ID,
		UserID:       o.UserID,
		UserName:     o.User.Name(),
		User2ID:      o.User2ID,
		ProductID:    o.ProductID,
		ProductName:  o.Product.Name,
		ProductPrice: o.ProductPrice,
		Quantity:     o.Quantity,
		FeesRate:     o.FeesRate,
		Fees:         fees,
		SubTotal:     subTotal,
		Total:        subTotal + fees,
		Paid:         o.Paid,
		CatalogID:    o.Product.CatalogID,
		CatalogName:  o.Product.Catalog.Name,
		CanModify:    o.CanModify(),
	}
}

// GetForUser retourne les commandes d'un utilisateur pour une distribution.
//
//	@Summary      Commandes d'un utilisateur
//	@Tags         orders
//	@Security     BearerAuth
//	@Produce      json
//	@Param        distributionId  query     int  true   "ID de la distribution"
//	@Param        userId          query     int  false  "ID utilisateur (admin)"
//	@Param        catalogId       query     int  false  "Filtrer par catalogue"
//	@Success      200  {object}  map[string]interface{}
//	@Router       /orders [get]
func (h *OrderHandler) GetForUser(c *gin.Context) {
	claims := middleware.GetClaims(c)

	// Par défaut : l'utilisateur connecté; un admin peut demander pour un autre user
	userID := claims.UserID
	if uidStr := c.Query("userId"); uidStr != "" {
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
			return
		}
		if uint(uid) != claims.UserID {
			var requester model.User
			if err := h.db.First(&requester, claims.UserID).Error; err != nil || !requester.IsAdmin() {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		userID = uint(uid)
	}

	distribID, err := strconv.Atoi(c.Query("distributionId"))
	if err != nil || distribID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "distributionId is required"})
		return
	}

	var catalogID *uint
	if cidStr := c.Query("catalogId"); cidStr != "" {
		cid, err := strconv.Atoi(cidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid catalogId"})
			return
		}
		uid := uint(cid)
		catalogID = &uid
	}

	svc := service.NewOrderService(h.db)
	orders, err := svc.GetUserOrders(userID, uint(distribID), catalogID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		out = append(out, prepareOrder(o))
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "orders": out})
}

// CreateOrUpdate crée ou met à jour les commandes d'un utilisateur.
//
//	@Summary      Créer/mettre à jour des commandes
//	@Tags         orders
//	@Security     BearerAuth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "userId, distributionId, catalogId, orders[]"
//	@Success      200   {object}  map[string]interface{}
//	@Router       /orders [post]
func (h *OrderHandler) CreateOrUpdate(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var payload struct {
		UserID         uint                  `json:"userId"`
		DistributionID uint                  `json:"distributionId" binding:"required"`
		CatalogID      *uint                 `json:"catalogId"`
		Orders         []service.OrderData   `json:"orders"         binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Résoudre l'utilisateur cible
	targetID := claims.UserID
	if payload.UserID != 0 && payload.UserID != claims.UserID {
		// Il faut être admin du groupe de la distribution (ou admin site-wide)
		var distrib model.Distribution
		if err := h.db.Preload("Catalog").First(&distrib, payload.DistributionID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "distribution not found"})
			return
		}
		ug := loadGroupAccess(h.db, claims.UserID, distrib.Catalog.GroupID)
		if ug == nil || !ug.IsGroupManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can edit orders for other users"})
			return
		}
		targetID = payload.UserID
	}

	var user model.User
	if err := h.db.First(&user, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	svc := service.NewOrderService(h.db)
	result, err := svc.CreateOrUpdateOrders(&user, payload.DistributionID, payload.CatalogID, payload.Orders)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out := make([]OrderResponse, 0, len(result))
	for _, o := range result {
		out = append(out, prepareOrder(o))
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "orders": out})
}
