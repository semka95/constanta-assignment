package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	transfers "payment-service/transfer/repository"
	"strconv"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type api struct {
	transfers *transfers.Queries
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	db, err := sql.Open("postgres", "postgres://dev:pass@127.0.0.1:5432/devdb?sslmode=disable")

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	queries := transfers.New(db)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	a := api{
		transfers: queries,
	}

	rand.Seed(time.Now().UnixNano())
	fmt.Println(rand.Float64())

	r.Post("/payment/create", a.createPayment)
	r.Put("/payment/{id}", a.updateStatus)
	r.Get("/payment/{id}", a.getStatus)
	r.Delete("/payment/{id}", a.cancelPayment)
	r.Get("/user/{id}/payments", a.getUserPaymentsByID)
	r.Get("/user/payments", a.getUserPaymentsByEmail)

	s := &http.Server{
		Addr:        ":3333",
		Handler:     r,
		ReadTimeout: 5 * time.Second,
		IdleTimeout: 30 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		timeout, _ := context.WithTimeout(ctx, 10*time.Second)
		s.Shutdown(timeout)
		cancel()
	}()
	s.ListenAndServe()
}

func (a *api) createPayment(w http.ResponseWriter, r *http.Request) {
	createTransfer := transfers.CreateTransferParams{}

	if err := render.DecodeJSON(r.Body, &createTransfer); err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid request body, can't decode it to transfer")
		return
	}

	transfer, err := a.transfers.CreateTransfer(r.Context(), createTransfer)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't create transfer record")
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, &transfer)
}

func (a *api) updateStatus(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	s := transfers.UpdateTransferStatusParams{}

	if err := render.DecodeJSON(r.Body, &s); err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid request body, can't decode it to transfer")
		return
	}

	s.ID = int64(paymentID)
	rows, err := a.transfers.UpdateTransferStatus(r.Context(), s)
	if err != nil {
		SendErrorJSON(w, r, http.StatusInternalServerError, err, "can't update transfer")
		return
	}
	if rows == 0 {
		SendErrorJSON(w, r, http.StatusBadRequest, errors.New(""), "can't update payment status, it has terminal status")
		return
	}

	render.Status(r, http.StatusNoContent)
}

func (a *api) getStatus(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	trStatus, err := a.transfers.GetTransferStatusByID(r.Context(), int64(paymentID))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "can't get transfer")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, JSON{"status": trStatus})
}

func (a *api) getUserPaymentsByID(w http.ResponseWriter, r *http.Request) {
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

	ts := make([]transfers.Transfer, 0)
	params := transfers.ListUserTransfersByIDParams{
		UserID: int64(userID),
		ID:     int64(cursor),
		Limit:  int32(limit),
	}
	ts, err = a.transfers.ListUserTransfersByID(r.Context(), params)
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "can't find transfer")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, ts)
}

func (a *api) getUserPaymentsByEmail(w http.ResponseWriter, r *http.Request) {
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

	params := transfers.ListUserTransfersByEmailParams{
		Email: email,
		ID:    int64(cursor),
		Limit: int32(limit),
	}
	ts := make([]transfers.Transfer, 0)
	ts, err = a.transfers.ListUserTransfersByEmail(r.Context(), params)
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "can't get transfer")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, ts)
}

func (a *api) cancelPayment(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		SendErrorJSON(w, r, http.StatusBadRequest, err, "invalid payment id")
		return
	}

	rows, err := a.transfers.DiscardTransfer(r.Context(), int64(paymentID))
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
