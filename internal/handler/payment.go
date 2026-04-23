package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/internal/service"
	"gorm.io/gorm"
)

type PaymentHandler struct{ db *gorm.DB }

func NewPaymentHandler(db *gorm.DB) *PaymentHandler {
	return &PaymentHandler{db: db}
}

// GetPaymentTypes retourne les modes de paiement autorisés pour un groupe.
//
//	@Summary      Modes de paiement
//	@Tags         payments
//	@Security     BearerAuth
//	@Produce      json
//	@Param        id   path      int  true  "Group ID"
//	@Success      200  {object}  map[string]interface{}
//	@Router       /groups/{id}/payment-types [get]
func (h *PaymentHandler) GetPaymentTypes(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	svc := service.NewPaymentService(h.db)
	types := svc.GetPaymentTypes(uint(groupID))
	c.JSON(http.StatusOK, gin.H{"success": true, "paymentTypes": types})
}

// CreatePayment enregistre un paiement pour un membre.
//
//	@Summary      Enregistrer un paiement
//	@Tags         payments
//	@Security     BearerAuth
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int     true  "Group ID"
//	@Param        body  body      object  true  "userId, amount, paymentType, name, relatedOpId"
//	@Success      200   {object}  map[string]interface{}
//	@Router       /groups/{id}/payments [post]
// CreatePayment enregistre un paiement pour un membre.
// Body: { "userId": X, "amount": Y, "paymentType": "cash", "name": "...", "relatedOpId": Z }
// Seuls les admins peuvent créer des paiements pour d'autres utilisateurs.
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	claims := middleware.GetClaims(c)

	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var payload struct {
		UserID      *uint   `json:"userId"`
		Amount      float64 `json:"amount"     binding:"required"`
		PaymentType string  `json:"paymentType" binding:"required"`
		Name        string  `json:"name"`
		RelatedOpID *uint   `json:"relatedOpId"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetID := claims.UserID
	if payload.UserID != nil && *payload.UserID != claims.UserID {
		var ug model.UserGroup
		if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).First(&ug).Error; err != nil || !ug.IsGroupManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can record payments for others"})
			return
		}
		targetID = *payload.UserID
	}

	name := payload.Name
	if name == "" {
		name = "Paiement"
	}

	svc := service.NewPaymentService(h.db)
	op, err := svc.MakePaymentOperation(targetID, uint(groupID), payload.PaymentType, payload.Amount, name, payload.RelatedOpID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "operation": prepareOperation(op)})
}

// GetOperations retourne les opérations d'un membre dans un groupe.
// GET /api/groups/:id/operations?userId=X&limit=50&offset=0
func (h *PaymentHandler) GetOperations(c *gin.Context) {
	claims := middleware.GetClaims(c)

	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	targetID := claims.UserID
	if uidStr := c.Query("userId"); uidStr != "" {
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
			return
		}
		if uint(uid) != claims.UserID {
			var ug model.UserGroup
			if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).First(&ug).Error; err != nil || !ug.IsGroupManager() {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		targetID = uint(uid)
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	var ops []model.Operation
	h.db.Where("user_id = ? AND group_id = ?", targetID, groupID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&ops)

	out := make([]gin.H, 0, len(ops))
	for _, op := range ops {
		out = append(out, prepareOperation(&op))
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "operations": out})
}

// ValidateDistribution valide une distribution (admin uniquement).
// POST /api/distributions/:id/validate
func (h *PaymentHandler) ValidateDistribution(c *gin.Context) {
	claims := middleware.GetClaims(c)

	distribID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid distribution id"})
		return
	}

	// Récupérer le groupe associé à ce MultiDistrib pour vérifier les droits
	var md model.MultiDistrib
	if err := h.db.First(&md, distribID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution not found"})
		return
	}
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, md.GroupID).First(&ug).Error; err != nil || !ug.IsGroupManager() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can validate distributions"})
		return
	}

	svc := service.NewPaymentService(h.db)
	if err := svc.ValidateDistribution(uint(distribID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func prepareOperation(op *model.Operation) gin.H {
	return gin.H{
		"id":          op.ID,
		"date":        op.CreatedAt,
		"amount":      op.Amount,
		"type":        op.Type,
		"description": op.Description,
		"pending":     op.Pending,
		"paymentType": op.PaymentType,
	}
}
