//go:build embed

package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var frontendFS embed.FS

func ServeEmbeddedFrontend() gin.HandlerFunc {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("failed to get dist subdirectory: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(distFS))

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/v1/") ||
			strings.HasPrefix(path, "/v1beta/") ||
			strings.HasPrefix(path, "/antigravity/") ||
			strings.HasPrefix(path, "/setup/") ||
			path == "/health" ||
			path == "/responses" {
			c.Next()
			return
		}

		// Only serve embedded frontend for safe navigation requests.
		// Non-GET/HEAD requests should fall through to API/routes (or 404), not index.html.
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		if file, err := distFS.Open(cleanPath); err == nil {
			_ = file.Close()
			setEmbeddedFrontendCacheHeaders(c, cleanPath)
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		// Only fall back to index.html for SPA routes (HTML navigation without a file extension).
		// If a static asset is missing (e.g. /assets/*.js), returning index.html would produce
		// "Expected a JavaScript module script but the server responded with a MIME type of text/html".
		if shouldServeEmbeddedIndexHTML(c.Request, cleanPath) {
			setEmbeddedFrontendCacheHeaders(c, "index.html")
			serveIndexHTML(c, distFS)
			return
		}

		c.Status(http.StatusNotFound)
		c.Abort()
	}
}

func setEmbeddedFrontendCacheHeaders(c *gin.Context, cleanPath string) {
	// Ensure index.html is always revalidated so users don't get stuck with a stale asset manifest.
	if cleanPath == "index.html" {
		c.Header("Cache-Control", "no-cache")
		return
	}

	// Vite builds fingerprint assets under /assets/; safe to cache aggressively.
	if strings.HasPrefix(cleanPath, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}
}

func shouldServeEmbeddedIndexHTML(r *http.Request, cleanPath string) bool {
	// Only treat it as an SPA route when the request is likely a browser navigation.
	accept := r.Header.Get("Accept")
	isHTMLNavigation := accept == "" || strings.Contains(accept, "text/html")
	if !isHTMLNavigation {
		return false
	}

	// If the path looks like a file request (has extension), do not fall back to index.html.
	// This avoids serving HTML for missing static assets.
	return path.Ext(cleanPath) == ""
}

func serveIndexHTML(c *gin.Context, fsys fs.FS) {
	file, err := fsys.Open("index.html")
	if err != nil {
		c.String(http.StatusNotFound, "Frontend not found")
		c.Abort()
		return
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read index.html")
		c.Abort()
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func HasEmbeddedFrontend() bool {
	_, err := frontendFS.ReadFile("dist/index.html")
	return err == nil
}
