package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type MemberHandler struct{ db *gorm.DB }

func NewMemberHandler(db *gorm.DB) *MemberHandler { return &MemberHandler{db: db} }

// List retourne les membres d'un groupe avec leur balance, paginés.
// Query params : ?page=N (défaut 1), ?perPage=M (défaut 10, plafonné 200).
// Retourne aussi waitingListCount pour la sidebar de la page Membres.
func (h *MemberHandler) List(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	claims := middleware.GetClaims(c)
	if loadGroupAccess(h.db, claims.UserID, uint(groupID)) == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "10"))
	if perPage < 1 || perPage > 200 {
		perPage = 10
	}

	var total int64
	h.db.Model(&model.UserGroup{}).Where("group_id = ?", groupID).Count(&total)

	var rows []struct {
		model.User
		Balance float64
		Rights  string
	}
	h.db.Model(&model.User{}).
		Select("users.*, user_groups.balance, user_groups.rights").
		Joins("JOIN user_groups ON user_groups.user_id = users.id").
		Where("user_groups.group_id = ?", groupID).
		Order("users.last_name").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Scan(&rows)

	type memberView struct {
		model.User
		Balance   float64 `json:"balance"`
		IsManager bool    `json:"isManager"`
		Address   string  `json:"address"`
	}
	members := make([]memberView, 0, len(rows))
	for _, r := range rows {
		ug := model.UserGroup{Rights: r.Rights}
		address := ""
		if r.Address1 != nil {
			address = *r.Address1
		}
		if r.ZipCode != nil {
			if address != "" {
				address += " "
			}
			address += *r.ZipCode
		}
		if r.City != nil {
			if address != "" {
				address += " "
			}
			address += *r.City
		}
		members = append(members, memberView{
			User:      r.User,
			Balance:   r.Balance,
			IsManager: ug.IsGroupManager(),
			Address:   address,
		})
	}

	totalPages := int(total) / perPage
	if int(total)%perPage != 0 {
		totalPages++
	}

	var waitingListCount int64
	h.db.Model(&model.WaitingList{}).
		Joins("JOIN catalogs ON catalogs.id = waiting_lists.catalog_id").
		Where("catalogs.group_id = ?", groupID).
		Count(&waitingListCount)

	c.JSON(http.StatusOK, gin.H{
		"members":          members,
		"total":            total,
		"totalPages":       totalPages,
		"page":             page,
		"perPage":          perPage,
		"waitingListCount": waitingListCount,
	})
}

// Add ajoute un utilisateur existant comme membre du groupe.
func (h *MemberHandler) Add(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	claims := middleware.GetClaims(c)
	ug := loadGroupAccess(h.db, claims.UserID, uint(groupID))
	if ug == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	if !ug.IsGroupManager() && !ug.HasRight(model.RightMembership) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can add members"})
		return
	}

	var payload struct {
		// Soit un userID existant, soit les infos pour créer un nouvel utilisateur
		UserID    *uint   `json:"userId"`
		FirstName string  `json:"firstName"`
		LastName  string  `json:"lastName"`
		Email     string  `json:"email"    binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User

	if payload.UserID != nil {
		// Associer un utilisateur existant
		if err := h.db.First(&user, payload.UserID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
	} else {
		// Chercher par email ou créer
		result := h.db.Where("email = ?", payload.Email).First(&user)
		if result.Error != nil {
			user = model.User{
				FirstName: payload.FirstName,
				LastName:  payload.LastName,
				Email:     payload.Email,
			}
			user.SetFlag(model.UserFlagEmailNotif24h)
			user.SetFlag(model.UserFlagEmailNotifOuverture)
			if err := h.db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	// Vérifier si déjà membre
	var existing model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", user.ID, groupID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		return
	}

	newUG := model.UserGroup{
		UserID:  user.ID,
		GroupID: uint(groupID),
	}
	if err := h.db.Create(&newUG).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": user, "membership": newUG})
}

// Remove retire un membre du groupe (ne supprime pas l'utilisateur).
func (h *MemberHandler) Remove(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	userID, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	claims := middleware.GetClaims(c)
	ug := loadGroupAccess(h.db, claims.UserID, uint(groupID))
	if ug == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	if !ug.IsGroupManager() && !ug.HasRight(model.RightMembership) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can remove members"})
		return
	}

	// Empêcher de se retirer soi-même si on est le seul admin
	if uint(userID) == claims.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove yourself from the group"})
		return
	}

	// Le superadmin global ne peut pas être retiré : il est administrateur
	// perpétuel de tous les groupes par construction.
	if isSiteAdmin(h.db, uint(userID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot remove the global super-admin"})
		return
	}

	result := h.db.Where("user_id = ? AND group_id = ?", userID, groupID).Delete(&model.UserGroup{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
