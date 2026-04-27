package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type FinanceHandler struct{ db *gorm.DB }

func NewFinanceHandler(db *gorm.DB) *FinanceHandler {
	return &FinanceHandler{db: db}
}

// GetBalance retourne la balance de l'utilisateur connecté dans un groupe.
//
//	@Summary      Balance personnelle
//	@Tags         finances
//	@Security     BearerAuth
//	@Produce      json
//	@Param        id   path      int  true  "Group ID"
//	@Success      200  {object}  map[string]interface{}
//	@Router       /groups/{id}/balance [get]
func (h *FinanceHandler) GetBalance(c *gin.Context) {
	claims := middleware.GetClaims(c)

	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).First(&ug).Error; err != nil {
		// Pas membre : un admin site-wide a accès mais n'a pas de balance.
		if isSiteAdmin(h.db, claims.UserID) {
			c.JSON(http.StatusOK, gin.H{"success": true, "balance": 0.0})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "balance": ug.Balance})
}

// GetGroupFinances retourne le résumé financier d'un groupe (admin uniquement).
//
//	@Summary      Tableau de bord finances (admin)
//	@Tags         finances
//	@Security     BearerAuth
//	@Produce      json
//	@Param        id   path      int  true  "Group ID"
//	@Success      200  {object}  map[string]interface{}
//	@Failure      403  {object}  map[string]string
//	@Router       /groups/{id}/finances [get]
// GetGroupFinances retourne le résumé financier d'un groupe (admin uniquement).
// Retourne : balance de chaque membre, total dettes, total paiements en attente.
func (h *FinanceHandler) GetGroupFinances(c *gin.Context) {
	claims := middleware.GetClaims(c)

	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	// Vérifier les droits admin (ou admin site-wide)
	ug := loadGroupAccess(h.db, claims.UserID, uint(groupID))
	if ug == nil || !ug.IsGroupManager() {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	// Membres avec leur balance
	var members []model.UserGroup
	h.db.Preload("User").
		Where("group_id = ?", groupID).
		Order("balance ASC").
		Find(&members)

	type memberBalance struct {
		UserID  uint    `json:"userId"`
		Name    string  `json:"name"`
		Email   string  `json:"email"`
		Balance float64 `json:"balance"`
	}

	out := make([]memberBalance, 0, len(members))
	var totalDebt, totalCredit float64
	for _, m := range members {
		out = append(out, memberBalance{
			UserID:  m.UserID,
			Name:    m.User.Name(),
			Email:   m.User.Email,
			Balance: m.Balance,
		})
		if m.Balance < 0 {
			totalDebt += m.Balance
		} else {
			totalCredit += m.Balance
		}
	}

	// Paiements en attente de validation
	var pendingCount int64
	var pendingSum float64
	h.db.Model(&model.Operation{}).
		Where("group_id = ? AND type = ? AND pending = 1", groupID, "Payment").
		Count(&pendingCount)
	h.db.Model(&model.Operation{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("group_id = ? AND type = ? AND pending = 1", groupID, "Payment").
		Scan(&pendingSum)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"members":      out,
		"totalDebt":    totalDebt,
		"totalCredit":  totalCredit,
		"pendingCount": pendingCount,
		"pendingSum":   pendingSum,
	})
}

// GetUserFinances retourne le détail financier d'un utilisateur dans un groupe.
// GET /api/groups/:id/finances/:userId
func (h *FinanceHandler) GetUserFinances(c *gin.Context) {
	claims := middleware.GetClaims(c)

	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	targetUID, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Un utilisateur peut voir ses propres finances ; un admin (groupe ou site-wide) peut voir celles de n'importe qui.
	if uint(targetUID) != claims.UserID {
		ug := loadGroupAccess(h.db, claims.UserID, uint(groupID))
		if ug == nil || !ug.IsGroupManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	var ug model.UserGroup
	if err := h.db.Preload("User").Where("user_id = ? AND group_id = ?", targetUID, groupID).First(&ug).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
		return
	}

	// 50 dernières opérations
	var ops []model.Operation
	h.db.Where("user_id = ? AND group_id = ?", targetUID, groupID).
		Order("created_at DESC").Limit(50).Find(&ops)

	opOut := make([]gin.H, 0, len(ops))
	for _, op := range ops {
		opOut = append(opOut, prepareOperation(&op))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"balance":    ug.Balance,
		"userId":     ug.UserID,
		"userName":   ug.User.Name(),
		"operations": opOut,
	})
}
