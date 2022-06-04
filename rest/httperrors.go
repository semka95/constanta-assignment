package main

import (
	"net/http"

	"github.com/go-chi/render"
)

type JSON map[string]interface{}

func SendErrorJSON(w http.ResponseWriter, r *http.Request, httpStatusCode int, err error, details string) {
	render.Status(r, httpStatusCode)
	render.JSON(w, r, JSON{"error": err.Error(), "details": details})
}
