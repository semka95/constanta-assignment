package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	postgres "github.com/semka95/payment-service/payment/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tCreatePayment = postgres.CreatePaymentParams{
	UserID:   1,
	Email:    "test@example.com",
	Amount:   decimal.NewFromFloat(123.42),
	Currency: "usd",
}

var tPayment = postgres.Payment{
	ID:            1,
	UserID:        1,
	Email:         "test@example.com",
	Amount:        decimal.NewFromFloat(123.42),
	Currency:      "usd",
	CreatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
	UpdatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
	PaymentStatus: postgres.ValidStatusNew,
}

var tPayments = []postgres.Payment{
	{
		ID:            1,
		UserID:        2,
		Email:         "test@example.com",
		Amount:        decimal.NewFromFloat(123.42),
		Currency:      "usd",
		CreatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
		UpdatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
		PaymentStatus: postgres.ValidStatusNew,
	},
	{
		ID:            2,
		UserID:        2,
		Email:         "test@example.com",
		Amount:        decimal.NewFromFloat(12223.42),
		Currency:      "eur",
		CreatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
		UpdatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
		PaymentStatus: postgres.ValidStatusNew,
	},
	{
		ID:            3,
		UserID:        2,
		Email:         "test@example.com",
		Amount:        decimal.NewFromFloat(123.423),
		Currency:      "rub",
		CreatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
		UpdatedAt:     time.Now().Truncate(time.Millisecond).UTC(),
		PaymentStatus: postgres.ValidStatusNew,
	},
}

var tUpdate = postgres.UpdatePaymentStatusParams{
	ID:            2,
	PaymentStatus: "success",
}

type jsonError struct {
	Details string `json:"details,omitempty"`
	Error   string `json:"error,omitempty"`
}

func TestCreatePayment(t *testing.T) {
	api := API{}
	req := new(http.Request)
	reqB, err := json.Marshal(tCreatePayment)
	require.NoError(t, err)

	cases := []struct {
		description    string
		mockedStore    *postgres.QuerierMock
		reqBody        *bytes.Buffer
		checkMockCalls func(tr *postgres.QuerierMock)
		checkResponse  func(rec *httptest.ResponseRecorder)
	}{
		{
			description: "success",
			mockedStore: &postgres.QuerierMock{
				CreatePaymentFunc: func(ctx context.Context, arg postgres.CreatePaymentParams) (postgres.Payment, error) {
					tr := postgres.Payment{
						ID:            1,
						UserID:        arg.UserID,
						Email:         arg.Email,
						Amount:        arg.Amount,
						Currency:      arg.Currency,
						CreatedAt:     tPayment.CreatedAt,
						UpdatedAt:     tPayment.UpdatedAt,
						PaymentStatus: postgres.ValidStatusNew,
					}
					return tr, nil
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.CreatePaymentCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				result := postgres.Payment{}
				err = json.NewDecoder(rec.Body).Decode(&result)
				require.NoError(t, err)
				assert.EqualValues(t, tPayment, result)
				assert.Equal(t, http.StatusCreated, rec.Code)
			},
		},
		{
			description:    "bad intput data",
			mockedStore:    &postgres.QuerierMock{},
			reqBody:        bytes.NewBuffer([]byte("bad data")),
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid request body, can't decode it to payment", jsonErr.Details)
				assert.Equal(t, "invalid character 'b' looking for beginning of value", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "repository server error",
			mockedStore: &postgres.QuerierMock{
				CreatePaymentFunc: func(ctx context.Context, arg postgres.CreatePaymentParams) (postgres.Payment, error) {
					return postgres.Payment{}, fmt.Errorf("can't create record")
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.CreatePaymentCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't create payment record", jsonErr.Details)
				assert.Equal(t, "can't create record", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			api.paymentStore = tc.mockedStore

			req = httptest.NewRequest("POST", "/payment", tc.reqBody)
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			api.createPayment(rec, req)

			tc.checkMockCalls(tc.mockedStore)

			tc.checkResponse(rec)
		})
	}
}

func TestGetStatus(t *testing.T) {
	api := API{}
	req := new(http.Request)
	c := chi.NewRouteContext()

	cases := []struct {
		description    string
		mockedStore    *postgres.QuerierMock
		id             string
		checkMockCalls func(tr *postgres.QuerierMock)
		checkResponse  func(rec *httptest.ResponseRecorder)
	}{
		{
			description: "success",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
			},
			id: "2",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				result := struct {
					Status string `json:"status"`
				}{}
				err := json.NewDecoder(rec.Body).Decode(&result)
				require.NoError(t, err)
				assert.EqualValues(t, postgres.ValidStatusNew, result.Status)
				assert.Equal(t, http.StatusOK, rec.Code)
			},
		},
		{
			description: "bad id",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			id:             "bad id",
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid payment id", jsonErr.Details)
				assert.Equal(t, "strconv.Atoi: parsing \"bad id\": invalid syntax", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "not found",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return "", sql.ErrNoRows
				},
			},
			id: "2",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "payment not found", jsonErr.Details)
				assert.Equal(t, "sql: no rows in result set", jsonErr.Error)
				assert.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			description: "repository server error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return "", fmt.Errorf("server error")
				},
			},
			id: "2",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't get payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			api.paymentStore = tc.mockedStore

			req = httptest.NewRequest("GET", "/payment/{id}", http.NoBody)
			req.Header.Set("Content-Type", "application/json")

			c.Reset()
			c.URLParams.Add("id", tc.id)
			req = req.WithContext((context.WithValue(req.Context(), chi.RouteCtxKey, c)))

			rec := httptest.NewRecorder()
			api.getStatus(rec, req)

			tc.checkMockCalls(tc.mockedStore)

			tc.checkResponse(rec)
		})
	}
}

func TestGetUserPaymentsByID(t *testing.T) {
	api := API{}
	req := new(http.Request)
	c := chi.NewRouteContext()

	cases := []struct {
		description    string
		mockedStore    *postgres.QuerierMock
		userID         string
		checkMockCalls func(tr *postgres.QuerierMock)
		checkResponse  func(rec *httptest.ResponseRecorder)
	}{
		{
			description: "success",
			mockedStore: &postgres.QuerierMock{
				ListUserPaymentsByIDFunc: func(ctx context.Context, arg postgres.ListUserPaymentsByIDParams) ([]postgres.Payment, error) {
					return tPayments, nil
				},
			},
			userID: "2",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.ListUserPaymentsByIDCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				result := make([]postgres.Payment, 0)
				err := json.NewDecoder(rec.Body).Decode(&result)
				require.NoError(t, err)
				assert.ElementsMatch(t, tPayments, result)
				assert.Equal(t, http.StatusOK, rec.Code)
			},
		},
		{
			description:    "bad user id",
			mockedStore:    &postgres.QuerierMock{},
			userID:         "bad id",
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid user id", jsonErr.Details)
				assert.Equal(t, "strconv.Atoi: parsing \"bad id\": invalid syntax", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "not found",
			mockedStore: &postgres.QuerierMock{
				ListUserPaymentsByIDFunc: func(ctx context.Context, arg postgres.ListUserPaymentsByIDParams) ([]postgres.Payment, error) {
					return []postgres.Payment{}, nil
				},
			},
			userID: "2",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.ListUserPaymentsByIDCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "no payments found", jsonErr.Details)
				assert.Equal(t, "no payments was found for 2 user id", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "repository server error",
			mockedStore: &postgres.QuerierMock{
				ListUserPaymentsByIDFunc: func(ctx context.Context, arg postgres.ListUserPaymentsByIDParams) ([]postgres.Payment, error) {
					return []postgres.Payment{}, fmt.Errorf("server error")
				},
			},
			userID: "2",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.ListUserPaymentsByIDCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't find payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			api.paymentStore = tc.mockedStore

			req = httptest.NewRequest("GET", "/user/{user_id}/payment", http.NoBody)
			req.Header.Set("Content-Type", "application/json")

			c.Reset()
			c.URLParams.Add("user_id", tc.userID)
			req = req.WithContext((context.WithValue(req.Context(), chi.RouteCtxKey, c)))

			rec := httptest.NewRecorder()
			api.getUserPaymentsByID(rec, req)

			tc.checkMockCalls(tc.mockedStore)

			tc.checkResponse(rec)
		})
	}
}

func TestGetUserPaymentsByIEmail(t *testing.T) {
	api := API{}
	req := new(http.Request)

	cases := []struct {
		description    string
		mockedStore    *postgres.QuerierMock
		email          string
		checkMockCalls func(tr *postgres.QuerierMock)
		checkResponse  func(rec *httptest.ResponseRecorder)
	}{
		{
			description: "success",
			mockedStore: &postgres.QuerierMock{
				ListUserPaymentsByEmailFunc: func(ctx context.Context, arg postgres.ListUserPaymentsByEmailParams) ([]postgres.Payment, error) {
					return tPayments, nil
				},
			},
			email: "test@example.com",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.ListUserPaymentsByEmailCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				result := make([]postgres.Payment, 0)
				err := json.NewDecoder(rec.Body).Decode(&result)
				require.NoError(t, err)
				assert.ElementsMatch(t, tPayments, result)
				assert.Equal(t, http.StatusOK, rec.Code)
			},
		},
		{
			description:    "empty email",
			mockedStore:    &postgres.QuerierMock{},
			email:          "",
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid email", jsonErr.Details)
				assert.Equal(t, "no email provided", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "not found",
			mockedStore: &postgres.QuerierMock{
				ListUserPaymentsByEmailFunc: func(ctx context.Context, arg postgres.ListUserPaymentsByEmailParams) ([]postgres.Payment, error) {
					return []postgres.Payment{}, nil
				},
			},
			email: "test@example.com",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.ListUserPaymentsByEmailCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "no payments found", jsonErr.Details)
				assert.Equal(t, "no payments was found for test@example.com email", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "repository server error",
			mockedStore: &postgres.QuerierMock{
				ListUserPaymentsByEmailFunc: func(ctx context.Context, arg postgres.ListUserPaymentsByEmailParams) ([]postgres.Payment, error) {
					return []postgres.Payment{}, fmt.Errorf("server error")
				},
			},
			email: "test@example.com",
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.ListUserPaymentsByEmailCalls())
				assert.Equal(t, 1, calls)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't find payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			api.paymentStore = tc.mockedStore

			req = httptest.NewRequest("GET", "/user/payment", http.NoBody)
			q := req.URL.Query()
			q.Add("email", tc.email)
			req.URL.RawQuery = q.Encode()
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			api.getUserPaymentsByEmail(rec, req)

			tc.checkMockCalls(tc.mockedStore)

			tc.checkResponse(rec)
		})
	}
}

func TestCancelPayment(t *testing.T) {
	db, mock, err := sqlmock.New()
	defer func() { _ = db.Close() }()
	require.NoError(t, err)

	api := API{db: db}
	req := new(http.Request)
	c := chi.NewRouteContext()

	cases := []struct {
		description    string
		mockedStore    *postgres.QuerierMock
		id             string
		expectSQL      func(mock sqlmock.Sqlmock)
		checkMockCalls func(tr *postgres.QuerierMock)
		checkResponse  func(rec *httptest.ResponseRecorder)
	}{
		{
			description: "success",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
				DiscardPaymentFunc: func(ctx context.Context, id int64) (int64, error) {
					return 1, nil
				},
			},
			id: "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.DiscardPaymentCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
			},
		},
		{
			description:    "bad id",
			mockedStore:    &postgres.QuerierMock{},
			id:             "bad id",
			expectSQL:      func(mock sqlmock.Sqlmock) {},
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid payment id", jsonErr.Details)
				assert.Equal(t, "strconv.Atoi: parsing \"bad id\": invalid syntax", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "begin transaction error",
			mockedStore: &postgres.QuerierMock{},
			id:          "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(fmt.Errorf("can't begin transaction"))
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't start transaction", jsonErr.Details)
				assert.Equal(t, "can't begin transaction", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
		{
			description: "not found",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return "", sql.ErrNoRows
				},
			},
			id: "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "payment not found", jsonErr.Details)
				assert.Equal(t, sql.ErrNoRows.Error(), jsonErr.Error)
				assert.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			description: "get status server error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return "", fmt.Errorf("server error")
				},
			},
			id: "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't update payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
		{
			description: "discard payment server error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
				DiscardPaymentFunc: func(ctx context.Context, id int64) (int64, error) {
					return -1, fmt.Errorf("server error")
				},
			},
			id: "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.DiscardPaymentCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't delete payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
		{
			description: "final status",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusSuccess, nil
				},
				DiscardPaymentFunc: func(ctx context.Context, id int64) (int64, error) {
					return 0, nil
				},
			},
			id: "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.DiscardPaymentCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't discard payment, it has final status", jsonErr.Details)
				assert.Equal(t, "can't discard payment, it has success status", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "commit transaction error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
				DiscardPaymentFunc: func(ctx context.Context, id int64) (int64, error) {
					return 1, nil
				},
			},
			id: "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit().WillReturnError(fmt.Errorf("can't commit transaction"))
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.DiscardPaymentCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't commit transaction", jsonErr.Details)
				assert.Equal(t, "can't commit transaction", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			api.paymentStore = tc.mockedStore

			req = httptest.NewRequest("DELETE", "/payment/{id}", http.NoBody)
			req.Header.Set("Content-Type", "application/json")

			c.Reset()
			c.URLParams.Add("id", tc.id)
			req = req.WithContext((context.WithValue(req.Context(), chi.RouteCtxKey, c)))

			tc.expectSQL(mock)

			rec := httptest.NewRecorder()
			api.cancelPayment(rec, req)

			tc.checkMockCalls(tc.mockedStore)

			tc.checkResponse(rec)
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	defer func() { _ = db.Close() }()
	require.NoError(t, err)

	api := API{db: db}
	req := new(http.Request)
	c := chi.NewRouteContext()
	reqB, err := json.Marshal(tUpdate)
	require.NoError(t, err)

	cases := []struct {
		description    string
		mockedStore    *postgres.QuerierMock
		reqBody        *bytes.Buffer
		id             string
		expectSQL      func(mock sqlmock.Sqlmock)
		checkMockCalls func(tr *postgres.QuerierMock)
		checkResponse  func(rec *httptest.ResponseRecorder)
	}{
		{
			description: "success",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
				UpdatePaymentStatusFunc: func(ctx context.Context, arg postgres.UpdatePaymentStatusParams) (int64, error) {
					return 1, nil
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			id:      "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.UpdatePaymentStatusCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
			},
		},
		{
			description:    "bad id",
			mockedStore:    &postgres.QuerierMock{},
			reqBody:        bytes.NewBuffer([]byte("")),
			id:             "bad id",
			expectSQL:      func(mock sqlmock.Sqlmock) {},
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid payment id", jsonErr.Details)
				assert.Equal(t, "strconv.Atoi: parsing \"bad id\": invalid syntax", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description:    "wrong body",
			mockedStore:    &postgres.QuerierMock{},
			reqBody:        bytes.NewBuffer([]byte("bad data")),
			id:             "2",
			expectSQL:      func(mock sqlmock.Sqlmock) {},
			checkMockCalls: func(tr *postgres.QuerierMock) {},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "invalid request body, can't decode it to payment", jsonErr.Details)
				assert.Equal(t, "invalid character 'b' looking for beginning of value", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "begin transaction error",
			mockedStore: &postgres.QuerierMock{},
			reqBody:     bytes.NewBuffer(reqB),
			id:          "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(fmt.Errorf("can't begin transaction"))
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't start transaction", jsonErr.Details)
				assert.Equal(t, "can't begin transaction", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
		{
			description: "not found",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return "", sql.ErrNoRows
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			id:      "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "payment not found", jsonErr.Details)
				assert.Equal(t, sql.ErrNoRows.Error(), jsonErr.Error)
				assert.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			description: "get status server error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return "", fmt.Errorf("server error")
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			id:      "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't update payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
		{
			description: "update payment server error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
				UpdatePaymentStatusFunc: func(ctx context.Context, arg postgres.UpdatePaymentStatusParams) (int64, error) {
					return -1, fmt.Errorf("server error")
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			id:      "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.UpdatePaymentStatusCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't update payment", jsonErr.Details)
				assert.Equal(t, "server error", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
		{
			description: "final status",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusFailure, nil
				},
				UpdatePaymentStatusFunc: func(ctx context.Context, arg postgres.UpdatePaymentStatusParams) (int64, error) {
					return 0, nil
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			id:      "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.UpdatePaymentStatusCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err = json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't update payment status", jsonErr.Details)
				assert.Equal(t, "can't update from success status to failure status", jsonErr.Error)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			description: "commit transaction error",
			mockedStore: &postgres.QuerierMock{
				GetPaymentStatusByIDFunc: func(ctx context.Context, id int64) (postgres.ValidStatus, error) {
					return postgres.ValidStatusNew, nil
				},
				UpdatePaymentStatusFunc: func(ctx context.Context, arg postgres.UpdatePaymentStatusParams) (int64, error) {
					return 1, nil
				},
			},
			reqBody: bytes.NewBuffer(reqB),
			id:      "2",
			expectSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit().WillReturnError(fmt.Errorf("can't commit transaction"))
			},
			checkMockCalls: func(tr *postgres.QuerierMock) {
				calls := len(tr.GetPaymentStatusByIDCalls())
				assert.Equal(t, 1, calls)
				calls = len(tr.UpdatePaymentStatusCalls())
				assert.Equal(t, 1, calls)
				err = mock.ExpectationsWereMet()
				assert.NoError(t, err)
			},
			checkResponse: func(rec *httptest.ResponseRecorder) {
				jsonErr := new(jsonError)
				err := json.NewDecoder(rec.Body).Decode(jsonErr)
				require.NoError(t, err)
				assert.Equal(t, "can't commit payment", jsonErr.Details)
				assert.Equal(t, "can't commit transaction", jsonErr.Error)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			api.paymentStore = tc.mockedStore

			req = httptest.NewRequest("PUT", "/payment/{id}", tc.reqBody)
			req.Header.Set("Content-Type", "application/json")

			c.Reset()
			c.URLParams.Add("id", tc.id)
			req = req.WithContext((context.WithValue(req.Context(), chi.RouteCtxKey, c)))

			tc.expectSQL(mock)

			rec := httptest.NewRecorder()
			api.updateStatus(rec, req)

			tc.checkMockCalls(tc.mockedStore)

			tc.checkResponse(rec)
		})
	}
}
