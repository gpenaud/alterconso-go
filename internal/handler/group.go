package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type GroupHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewGroupHandler(db *gorm.DB, cfg *config.Config) *GroupHandler {
	return &GroupHandler{db: db, cfg: cfg}
}

// List retourne les groupes dont l'utilisateur est membre.
// Les superadmins site-wide voient tous les groupes existants.
//
//	@Summary      Liste des groupes
//	@Tags         groups
//	@Security     BearerAuth
//	@Produce      json
//	@Success      200  {array}   model.Group
//	@Router       /groups [get]
func (h *GroupHandler) List(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var groups []model.Group
	if isSiteAdmin(h.db, claims.UserID) {
		h.db.Order("name").Find(&groups)
		c.JSON(http.StatusOK, groups)
		return
	}

	var userGroups []model.UserGroup
	h.db.Where("user_id = ?", claims.UserID).Find(&userGroups)

	groupIDs := make([]uint, len(userGroups))
	for i, ug := range userGroups {
		groupIDs[i] = ug.GroupID
	}

	h.db.Where("id IN ?", groupIDs).Order("name").Find(&groups)

	c.JSON(http.StatusOK, groups)
}

// Get retourne un groupe par son ID.
//
//	@Summary      Détail d'un groupe
//	@Tags         groups
//	@Security     BearerAuth
//	@Produce      json
//	@Param        id   path      int  true  "Group ID"
//	@Success      200  {object}  model.Group
//	@Failure      403  {object}  map[string]string
//	@Failure      404  {object}  map[string]string
//	@Router       /groups/{id} [get]
func (h *GroupHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	claims := middleware.GetClaims(c)

	// Vérifier que l'utilisateur est membre du groupe (ou admin site-wide)
	if loadGroupAccess(h.db, claims.UserID, uint(id)) == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return
	}

	var group model.Group
	if err := h.db.Preload("Contact").Preload("MainPlace").First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	c.JSON(http.StatusOK, group)
}

// Create crée un nouveau groupe.
//
//	@Summary      Créer un groupe
//	@Tags         groups
//	@Security     BearerAuth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "name, groupType, regOption"
//	@Success      201   {object}  model.Group
//	@Router       /groups [post]
func (h *GroupHandler) Create(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var payload struct {
		Name      string           `json:"name"      binding:"required"`
		GroupType model.GroupType  `json:"groupType"`
		RegOption model.RegOption  `json:"regOption"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := model.Group{
		Name:      payload.Name,
		GroupType: payload.GroupType,
		RegOption: payload.RegOption,
	}
	group.SetFlag(model.GroupFlagShopMode)
	group.SetFlag(model.GroupFlagCagetteNetwork)

	if err := h.db.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ajouter le créateur comme admin du groupe
	ug := model.UserGroup{
		UserID:  claims.UserID,
		GroupID: group.ID,
	}
	h.db.Create(&ug)

	c.JSON(http.StatusCreated, group)
}

func (h *GroupHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	claims := middleware.GetClaims(c)

	// Vérifier que l'utilisateur est admin du groupe (ou admin site-wide)
	ug := loadGroupAccess(h.db, claims.UserID, uint(id))
	if ug == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	if !ug.IsGroupManager() {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a group admin"})
		return
	}

	var group model.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var payload struct {
		Name       string  `json:"name"`
		TxtIntro   *string `json:"txtIntro"`
		TxtHome    *string `json:"txtHome"`
		TxtDistrib *string `json:"txtDistrib"`
		ExtURL     *string `json:"extUrl"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&group).Updates(map[string]interface{}{
		"name":        payload.Name,
		"txt_intro":   payload.TxtIntro,
		"txt_home":    payload.TxtHome,
		"txt_distrib": payload.TxtDistrib,
		"ext_url":     payload.ExtURL,
	})

	c.JSON(http.StatusOK, group)
}
