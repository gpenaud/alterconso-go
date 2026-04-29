package handler

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type FileHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewFileHandler(db *gorm.DB, cfg *config.Config) *FileHandler {
	return &FileHandler{db: db, cfg: cfg}
}

// makeSign reproduit la logique Haxe : id+"_"+md5(id+key)
func makeSign(id uint, key string) string {
	raw := fmt.Sprintf("%d%s", id, key)
	return fmt.Sprintf("%d_%x", id, md5.Sum([]byte(raw)))
}

// GET /file/:sign  — ex: /file/42_abcdef1234....png
func (h *FileHandler) ServeFile(c *gin.Context) {
	sign := c.Param("sign")
	// Enlever l'extension
	ext := ""
	if dot := strings.LastIndex(sign, "."); dot >= 0 {
		ext = sign[dot+1:]
		sign = sign[:dot]
	}

	// Extraire l'id (partie avant le _)
	parts := strings.SplitN(sign, "_", 2)
	if len(parts) != 2 {
		c.Status(http.StatusNotFound)
		return
	}
	id64, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	id := uint(id64)

	// Vérifier la signature
	expected := makeSign(id, h.cfg.Key)
	if sign != expected {
		c.Status(http.StatusForbidden)
		return
	}

	var file model.File
	if err := h.db.First(&file, id).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	contentType := "image/png"
	switch strings.ToLower(ext) {
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "gif":
		contentType = "image/gif"
	case "webp":
		contentType = "image/webp"
	case "pdf":
		contentType = "application/pdf"
	}

	// L'URL contient un hash signé (id + HMAC). Quand un fichier est remplacé,
	// un nouveau row File est créé avec un nouvel ID → nouvelle URL. L'URL est
	// donc content-immutable : on peut la cacher 1 an sans revalidation.
	// `immutable` interdit au navigateur d'envoyer If-None-Match pendant la
	// durée de vie du cache (sinon round-trip inutile).
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	if contentType == "application/pdf" {
		c.Header("Content-Disposition", "inline; filename=\""+file.Name+"\"")
	}
	c.Data(http.StatusOK, contentType, file.Data)
}

// FileURL génère l'URL publique d'un File (utilisable dans les templates/handlers)
func FileURL(id uint, key, name string) string {
	sign := makeSign(id, key)
	ext := "png"
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		ext = name[dot+1:]
	}
	return fmt.Sprintf("/file/%s.%s", sign, ext)
}
