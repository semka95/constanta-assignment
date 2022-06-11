package api

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"

	paymentModel "github.com/semka95/payment-service/payment/repository"
)

// API represents payment rest api
type API struct {
	paymentStore paymentModel.Querier
	db           *sql.DB
	errorChance  float64
}

// NewRouter creates payment api router
func (a *API) NewRouter(paymentStore paymentModel.Querier, db *sql.DB, errorChance float64) chi.Router {
	a.paymentStore = paymentStore
	a.db = db
	a.errorChance = errorChance

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
		rapi.Get("/user/{user_id}/payment", a.getUserPaymentsByID)
		rapi.Get("/user/payment", a.getUserPaymentsByEmail)
	})

	return r
}

// POST /payment- creates new payment
func (a *API) createPayment(w http.ResponseWriter, r *http.Request) {
	createPayment := paymentModel.CreatePaymentParams{}

	if err := render.DecodeJSON(r.Body, &createPayment); err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid request body, can't decode it to payment")
		return
	}

	createPayment.PaymentStatus = paymentModel.ValidStatusNew
	if 1-rand.Float64() <= a.errorChance {
		createPayment.PaymentStatus = paymentModel.ValidStatusError
	}

	payment, err := a.paymentStore.CreatePayment(r.Context(), createPayment)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't create payment record")
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, &payment)
}

// PUT /payment/{id} - updates payment status
func (a *API) updateStatus(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	s := paymentModel.UpdatePaymentStatusParams{}

	if err = render.DecodeJSON(r.Body, &s); err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid request body, can't decode it to payment")
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't start transaction")
		return
	}
	defer tx.Rollback()
	status, err := a.paymentStore.GetPaymentStatusByID(r.Context(), int64(paymentID))
	if errors.Is(err, sql.ErrNoRows) {
		SendErrorJSON(w, r, http.StatusNotFound, err, "payment not found")
		return
	}
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't update payment")
		return
	}

	s.ID = int64(paymentID)
	rows, err := a.paymentStore.UpdatePaymentStatus(r.Context(), s)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't update payment")
		return
	}
	if rows == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, fmt.Errorf("can't update from %s status to %s status", s.PaymentStatus, status), "can't update payment status")
		return
	}

	if err := tx.Commit(); err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't commit payment")
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

	trStatus, err := a.paymentStore.GetPaymentStatusByID(r.Context(), int64(paymentID))
	if errors.Is(err, sql.ErrNoRows) {
		SendErrorJSON(w, r, http.StatusNotFound, err, "payment not found")
		return
	}
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't get payment")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, JSON{"status": trStatus})
}

// GET /user/{id}/payment?limit=5&cursor=0 - returns payments by user id
func (a *API) getUserPaymentsByID(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(chi.URLParam(r, "user_id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid user id")
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

	params := paymentModel.ListUserPaymentsByIDParams{
		UserID: int64(userID),
		ID:     int64(cursor),
		Limit:  int32(limit),
	}
	ts, err := a.paymentStore.ListUserPaymentsByID(r.Context(), params)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't find payment")
		return
	}
	if len(ts) == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, fmt.Errorf("no payments was found for %d user id", userID), "no payments found")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, ts)
}

// GET /user/payment?email=userEmail&limit=5&cursor=0 - returns payments by user email
func (a *API) getUserPaymentsByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		SendErrorJSON(w, r, http.StatusBadRequest, errors.New("no email provided"), "invalid email")
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

	params := paymentModel.ListUserPaymentsByEmailParams{
		Email: email,
		ID:    int64(cursor),
		Limit: int32(limit),
	}
	ts, err := a.paymentStore.ListUserPaymentsByEmail(r.Context(), params)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't find payment")
		return
	}
	if len(ts) == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, fmt.Errorf("no payments was found for %s email", email), "no payments found")
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

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't start transaction")
		return
	}
	defer tx.Rollback()

	status, err := a.paymentStore.GetPaymentStatusByID(r.Context(), int64(paymentID))
	if errors.Is(err, sql.ErrNoRows) {
		SendErrorJSON(w, r, http.StatusNotFound, err, "payment not found")
		return
	}
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't update payment")
		return
	}

	rows, err := a.paymentStore.DiscardPayment(r.Context(), int64(paymentID))
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't delete payment")
		return
	}
	if rows == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, fmt.Errorf("can't discard payment, it has %s status", status), "can't discard payment, it has terminal status")
		return
	}

	if err := tx.Commit(); err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't commit transaction")
		return
	}

	render.Status(r, http.StatusNoContent)
}
