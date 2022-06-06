package transfer

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"

	transferModel "payment-service/transfer/repository"
)

// API represents transfer rest api
type API struct {
	transferStore transferModel.Querier
}

// NewRouter creates transfer api router
func (a *API) NewRouter(transferStore transferModel.Querier) chi.Router {
	a.transferStore = transferStore

	r := chi.NewRouter()
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	})
	r.Use(middleware.Recoverer, corsMiddleware.Handler)
	r.Route("/api/v1", func(rapi chi.Router) {
		rapi.Post("/payment", a.createPayment)
		rapi.Put("/payment/{id}", a.updateStatus)
		rapi.Get("/payment/{id}", a.getStatus)
		rapi.Delete("/payment/{id}", a.cancelPayment)
		rapi.Get("/user/{id}/payment", a.getUserPaymentsByID)
		rapi.Get("/user/payment", a.getUserPaymentsByEmail)
	})

	return r
}

// POST /payment- creates new payment
func (a *API) createPayment(w http.ResponseWriter, r *http.Request) {
	createTransfer := transferModel.CreateTransferParams{}

	if err := render.DecodeJSON(r.Body, &createTransfer); err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid request body, can't decode it to transfer")
		return
	}

	transfer, err := a.transferStore.CreateTransfer(r.Context(), createTransfer)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't create transfer record")
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, &transfer)
}

// PUT /payment/{id} - updates payment status
func (a *API) updateStatus(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	s := transferModel.UpdateTransferStatusParams{}

	if err = render.DecodeJSON(r.Body, &s); err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid request body, can't decode it to transfer")
		return
	}

	s.ID = int64(paymentID)
	rows, err := a.transferStore.UpdateTransferStatus(r.Context(), s)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't update transfer")
		return
	}
	if rows == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, errors.New("hello"), "can't update payment status, it has terminal status")
		return
	}

	render.Status(r, http.StatusNoContent)
}

// GET /payment/{id} - returns payment status
func (a *API) getStatus(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	trStatus, err := a.transferStore.GetTransferStatusByID(r.Context(), int64(paymentID))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "can't get transfer")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, JSON{"status": trStatus})
}

// GET /user/{id}/payment?limit=5&cursor=0 - returns payments by user id
func (a *API) getUserPaymentsByID(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 10
	}
	cursor, err := strconv.Atoi(r.URL.Query().Get("cursor"))
	if err != nil {
		cursor = 0
	}

	params := transferModel.ListUserTransfersByIDParams{
		UserID: int64(userID),
		ID:     int64(cursor),
		Limit:  int32(limit),
	}
	ts, err := a.transferStore.ListUserTransfersByID(r.Context(), params)
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "can't find transfer")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, ts)
}

// GET /user/payment?email=userEmail&limit=5&cursor=0 - returns payments by user email
func (a *API) getUserPaymentsByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		SendErrorJSON(w, r, http.StatusBadRequest, errors.New(""), "invalid email")
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 10
	}
	cursor, err := strconv.Atoi(r.URL.Query().Get("cursor"))
	if err != nil {
		cursor = 0
	}

	params := transferModel.ListUserTransfersByEmailParams{
		Email: email,
		ID:    int64(cursor),
		Limit: int32(limit),
	}

	ts, err := a.transferStore.ListUserTransfersByEmail(r.Context(), params)
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "can't get transfer")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, ts)
}

// DELETE /payment/{id} - delete payment
func (a *API) cancelPayment(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	rows, err := a.transferStore.DiscardTransfer(r.Context(), int64(paymentID))
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't delete transfer")
		return
	}
	if rows == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, errors.New(""), "can't discard payment, it has terminal status")
		return
	}

	render.Status(r, http.StatusNoContent)
}
