package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gpenaud/alterconso/internal/config"
)

type Claims struct {
	UserID  uint `json:"userId"`
	GroupID uint `json:"groupId"`
	jwt.RegisteredClaims
}

const claimsKey = "claims"

// Auth vérifie le JWT dans le header Authorization: Bearer <token> ou dans le cookie "token".
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := parseToken(tokenStr, cfg.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(claimsKey, claims)
		c.Next()
	}
}

// PageAuth protège les pages HTML : redirige vers /user/login si non authentifié.
func PageAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			redirect := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(http.StatusFound, "/user/login?__redirect="+redirect)
			c.Abort()
			return
		}

		claims, err := parseToken(tokenStr, cfg.JWTSecret)
		if err != nil {
			redirect := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(http.StatusFound, "/user/login?__redirect="+redirect)
			c.Abort()
			return
		}

		c.Set(claimsKey, claims)
		c.Next()
	}
}

// extractToken lit le JWT depuis le header Authorization ou le cookie "token".
func extractToken(c *gin.Context) string {
	if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	if cookie, err := c.Cookie("token"); err == nil && cookie != "" {
		return cookie
	}
	return ""
}

func parseToken(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}

// GetClaims extrait les claims JWT depuis le contexte Gin.
func GetClaims(c *gin.Context) *Claims {
	v, _ := c.Get(claimsKey)
	claims, _ := v.(*Claims)
	return claims
}
