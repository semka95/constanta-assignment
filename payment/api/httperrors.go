package api

import (
	"net/http"

	"github.com/go-chi/render"
	"go.uber.org/zap"
)

// JSON is a map alias
type JSON map[string]interface{}

// SendErrorJSON sends error as json
func SendErrorJSON(w http.ResponseWriter, r *http.Request, httpStatusCode int, err error, details string) {
	zap.L().Warn(details, zap.Error(err), zap.Int("httpStatusCode", httpStatusCode), zap.String("url", r.URL.String()), zap.String("method", r.Method))
	render.Status(r, httpStatusCode)
	render.JSON(w, r, JSON{"error": err.Error(), "details": details})
}
