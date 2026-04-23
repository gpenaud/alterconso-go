package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
	GroupID  uint   `json:"groupId"`
}

type loginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

// Login authentifie un utilisateur et retourne un token JWT.
//
//	@Summary      Connexion
//	@Description  Authentifie l'utilisateur avec email + mot de passe et retourne un JWT.
//	@Tags         auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      loginRequest   true  "Identifiants"
//	@Success      200   {object}  loginResponse
//	@Failure      400   {object}  map[string]string
//	@Failure      401   {object}  map[string]string
//	@Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.db.Where("email = ? OR email2 = ?", req.Email, req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants incorrects"})
		return
	}

	if !user.CheckPassword(req.Password, h.cfg.Key) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants incorrects"})
		return
	}

	// Mise à jour de la date de dernière connexion
	now := time.Now()
	h.db.Model(&user).Update("last_login", now)

	// Génération du JWT
	claims := &middleware.Claims{
		UserID:  user.ID,
		GroupID: req.GroupID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(
				time.Duration(h.cfg.JWTExpiryHours) * time.Hour,
			)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: signed, User: user})
}

// Logout déconnecte l'utilisateur (côté client).
//
//	@Summary      Déconnexion
//	@Tags         auth
//	@Security     BearerAuth
//	@Success      200  {object}  map[string]bool
//	@Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// Avec JWT stateless, le logout est géré côté client (suppression du token).
	// Ici on pourrait alimenter une blacklist Redis si nécessaire.
	c.JSON(http.StatusOK, gin.H{"success": true})
}
