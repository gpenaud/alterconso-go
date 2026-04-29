package handler

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// StaticPrecompressed retourne un handler Gin qui sert un dossier statique
// en privilégiant les variantes pré-compressées au build :
//
//   1. Si le client supporte brotli ET <fichier>.br existe → sert .br
//      avec Content-Encoding: br
//   2. Sinon si le client supporte gzip ET <fichier>.gz existe → sert .gz
//      avec Content-Encoding: gzip
//   3. Sinon sert <fichier> tel quel (404 s'il n'existe pas).
//
// Le Content-Type est dérivé de l'extension du fichier ORIGINAL (sans
// .br/.gz), pour que le browser interprète correctement la charge utile.
//
// `urlPrefix` est le chemin URL qui mappe vers `dir` (ex "/css", "www/css").
// Le handler doit être enregistré avec un wildcard : r.GET(urlPrefix+"/*filepath", ...).
func StaticPrecompressed(urlPrefix, dir string) gin.HandlerFunc {
	dir = filepath.Clean(dir)
	return func(c *gin.Context) {
		// Path relatif au dossier statique, nettoyé pour éviter les remontées.
		rel := strings.TrimPrefix(c.Request.URL.Path, urlPrefix)
		rel = path.Clean("/" + rel)
		rel = strings.TrimPrefix(rel, "/")
		full := filepath.Join(dir, filepath.FromSlash(rel))
		// Garde-fou anti-traversal : full doit rester sous dir.
		if !strings.HasPrefix(full, dir+string(filepath.Separator)) && full != dir {
			c.Status(http.StatusForbidden)
			return
		}

		ae := c.GetHeader("Accept-Encoding")
		ct := mime.TypeByExtension(filepath.Ext(rel))

		if strings.Contains(ae, "br") {
			if served := serveCompressed(c, full+".br", "br", ct); served {
				return
			}
		}
		if strings.Contains(ae, "gzip") {
			if served := serveCompressed(c, full+".gz", "gzip", ct); served {
				return
			}
		}
		// Fallback sans compression.
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			c.File(full)
			return
		}
		c.Status(http.StatusNotFound)
	}
}

// SPAFallback retourne un handler qui sert une SPA depuis `dir` :
//   - Si le path correspond à un fichier existant sous `dir`, il est servi
//     directement.
//   - Sinon (route client-side gérée par React Router), `index.html` est
//     renvoyé pour que la SPA prenne le relais.
//
// `urlPrefix` est l'URL sous laquelle la SPA est montée (ex "/shop2"). Doit
// être enregistré avec un wildcard : r.GET(urlPrefix+"/*filepath", ...).
func SPAFallback(urlPrefix, dir string) gin.HandlerFunc {
	dir = filepath.Clean(dir)
	indexHTML := filepath.Join(dir, "index.html")
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Request.URL.Path, urlPrefix)
		rel = path.Clean("/" + rel)
		rel = strings.TrimPrefix(rel, "/")
		// Racine ou route SPA → index.html
		if rel == "" {
			c.File(indexHTML)
			return
		}
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if !strings.HasPrefix(full, dir+string(filepath.Separator)) && full != dir {
			c.Status(http.StatusForbidden)
			return
		}
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			c.File(full)
			return
		}
		// Route SPA inconnue côté serveur → laisse React Router gérer.
		c.File(indexHTML)
	}
}

func serveCompressed(c *gin.Context, path, encoding, contentType string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Content-Encoding", encoding)
	c.Header("Vary", "Accept-Encoding")
	c.File(path)
	return true
}
