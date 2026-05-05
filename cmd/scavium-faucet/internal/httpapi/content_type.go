package httpapi

import (
	"mime"
	"net/http"
	"strings"
)

func requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "content type must be application/json", nil)
		return false
	}
	return true
}
