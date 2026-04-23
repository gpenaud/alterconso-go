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

type UserHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewUserHandler(db *gorm.DB, cfg *config.Config) *UserHandler {
	return &UserHandler{db: db, cfg: cfg}
}

// Me retourne le profil de l'utilisateur connecté.
func (h *UserHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateMe met à jour le profil de l'utilisateur connecté.
func (h *UserHandler) UpdateMe(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var payload struct {
		FirstName string  `json:"firstName"`
		LastName  string  `json:"lastName"`
		Phone     *string `json:"phone"`
		Address1  *string `json:"address1"`
		Address2  *string `json:"address2"`
		ZipCode   *string `json:"zipCode"`
		City      *string `json:"city"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&user).Updates(map[string]interface{}{
		"first_name": payload.FirstName,
		"last_name":  payload.LastName,
		"phone":      payload.Phone,
		"address1":   payload.Address1,
		"address2":   payload.Address2,
		"zip_code":   payload.ZipCode,
		"city":       payload.City,
	})

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	claims := middleware.GetClaims(c)

	// Un utilisateur ne peut voir que son propre profil, sauf admin
	if uint(id) != claims.UserID {
		var requester model.User
		if err := h.db.First(&requester, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if !requester.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	claims := middleware.GetClaims(c)
	if uint(id) != claims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Champs modifiables par l'utilisateur lui-même
	var payload struct {
		FirstName  string  `json:"firstName"`
		LastName   string  `json:"lastName"`
		Phone      *string `json:"phone"`
		Address1   *string `json:"address1"`
		Address2   *string `json:"address2"`
		ZipCode    *string `json:"zipCode"`
		City       *string `json:"city"`
		Lang       string  `json:"lang"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&user).Updates(map[string]interface{}{
		"first_name": payload.FirstName,
		"last_name":  payload.LastName,
		"phone":      payload.Phone,
		"address1":   payload.Address1,
		"address2":   payload.Address2,
		"zip_code":   payload.ZipCode,
		"city":       payload.City,
		"lang":       payload.Lang,
	})

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) List(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var requester model.User
	if err := h.db.First(&requester, claims.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if !requester.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var users []model.User
	h.db.Order("last_name").Find(&users)
	c.JSON(http.StatusOK, users)
}
