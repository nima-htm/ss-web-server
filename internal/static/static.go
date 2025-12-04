package static

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticFileHandler handles serving static files
type StaticFileHandler struct {
	root  string
	index string
}

// NewStaticFileHandler creates a new static file handler
func NewStaticFileHandler(root string, index string) *StaticFileHandler {
	if index == "" {
		index = "index.html"
	}
	return &StaticFileHandler{
		root:  root,
		index: index,
	}
}

// Handle handles the static file request
func (s *StaticFileHandler) Handle(w http.ResponseWriter, r *http.Request) {
	requestPath := filepath.Clean(r.URL.Path)

	filePath := filepath.Join(s.root, requestPath)

	if info, err := os.Stat(filePath); err == nil && info.IsDir() {
		indexPath := filepath.Join(filePath, s.index)
		if indexInfo, err := os.Stat(indexPath); err == nil && !indexInfo.IsDir() {
			http.ServeFile(w, r, indexPath)
			return
		} else {
			http.NotFound(w, r)
			return
		}
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	if info, err := os.Stat(filePath); err == nil && info.IsDir() {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	http.ServeFile(w, r, filePath)
}

// GetMimeType returns the MIME type for a file based on its extension
func GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js", ".javascript":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".txt":
		return "text/plain"
	case ".xml":
		return "text/xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
