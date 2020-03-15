package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"

	"github.com/peppinux/dero-merchant/httperror"
)

// PingGetHandler handles GET requests to /api/v1/ping
func PingGetHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ping": "pong",
	})
}

type paymentPostRequest struct {
	Currency string  `json:"currency" binding:"required,max=4,min=3"`
	Amount   float64 `json:"amount" binding:"required"`
}

var paymentPostFieldsErrors = map[string]string{
	"Currency": ErrInvalidCurrency.Error(),
	"Amount":   ErrInvalidAmount.Error(),
}

// PaymentPostHandler handles POST requests to /api/v1/payment
func PaymentPostHandler(c *gin.Context) {
	// Get and validate request params
	var req paymentPostRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			httperror.Send(c, http.StatusBadRequest, "Invalid request params")
			return
		}

		for _, err := range errs {
			httperror.Send(c, http.StatusUnprocessableEntity, paymentPostFieldsErrors[err.Field()])
			return
		}
	}

	storeID := c.MustGet("storeID").(int)

	// Create Payment
	p, w, errCode, err := CreateNewPayment(req.Currency, req.Amount, storeID)
	if err != nil {
		if errCode == http.StatusInternalServerError {
			httperror.Send500(c, err, "Error creating new payment")
			return
		}

		httperror.Send(c, errCode, err.Error())
		return
	}

	// Insert Payment into DB
	err = p.Insert()
	if httperror.Send500IfErr(c, err, "Error inserting Payment into DB") != nil {
		return
	}

	// Add Payment to wallet's pending payments
	err = w.AddPendingPayment(p.PaymentID, p.AtomicDeroAmount)
	if httperror.Send500IfErr(c, err, "Error adding pending payment to wallet") != nil {
		return
	}

	c.JSON(http.StatusCreated, p)
}

// PaymentGetHandler handles GET requests to /api/v1/payment/:payment_id
func PaymentGetHandler(c *gin.Context) {
	paymentID := c.Param("payment_id")
	storeID := c.MustGet("storeID").(int)

	p, errCode, err := FetchPaymentFromID(paymentID, storeID)
	if err != nil {
		if errCode == http.StatusInternalServerError {
			httperror.Send500(c, err, "Error fetching payment from database")
			return
		}

		httperror.Send(c, errCode, err.Error())
		return
	}

	c.JSON(http.StatusOK, p)
}

type paymentsPostRequest []string
type paymentsPostResponse []*Payment

// PaymentsPostHandler handles POST requests to /api/v1/payments
func PaymentsPostHandler(c *gin.Context) {
	storeID := c.MustGet("storeID").(int)

	var (
		paymentIDs paymentsPostRequest
		payments   paymentsPostResponse
	)

	// Get request params
	err := c.ShouldBindJSON(&paymentIDs)
	if err != nil {
		httperror.Send(c, http.StatusBadRequest, "Invalid request params")
		return
	}

	if len(paymentIDs) == 0 {
		httperror.Send(c, http.StatusBadRequest, "No Payment IDs submitted")
		return
	}

	payments, errCode, err := FetchPaymentsFromIDs(paymentIDs, storeID)
	if err != nil {
		if errCode == http.StatusInternalServerError {
			httperror.Send500(c, err, "Error fetching payments from database")
			return
		}

		httperror.Send(c, errCode, err.Error())
		return
	}

	if len(payments) == 0 {
		httperror.Send(c, http.StatusNotFound, "Payments not found")
		return
	}

	c.JSON(http.StatusOK, payments)
}

type paymentsGetRequest struct {
	// Pagination
	Limit int `form:"limit,default=0" binding:"min=0"`
	Page  int `form:"page,default=1" binding:"min=1"`
	// Sorting
	SortBy  string `form:"sort_by,default=creation_time" binding:"eq=|eq=currency_amount|eq=exchange_rate|eq=atomic_dero_amount|eq=creation_time"`
	OrderBy string `form:"order_by,default=desc" binding:"eq=|eq=asc|eq=desc"`
	// Filtering
	Status   string `form:"status,default=" binding:"eq=|eq=pending|eq=paid|eq=expired|eq=error"`
	Currency string `form:"currency,default=" binding:"max=4"`
}

var paymentsGetFieldsErrors = map[string]string{
	"Limit":    "Query param 'limit' not valid. Allowed values: (empty) or min 0",
	"Page":     "Query param 'page' not valid. Allowed values: (empty) or min 1",
	"SortBy":   "Query param 'sort_by' not valid. Allowed values: (empty), creation_time, currency_amount, exchange_rate, atomic_dero_amount",
	"OrderBy":  "Query param 'order_by' not valid. Allowed values: (empty), asc, desc",
	"Status":   "Query param 'status' not valid. Allowed values: (empty), pending, paid, expired, error",
	"Currency": "Query param 'currency' not valid. Allowed values: (empty) or max 4 characters",
}

type paymentsGetResponse struct {
	// Pagination
	Limit int `json:"limit"`
	Page  int `json:"page,omitempty"`
	// Total number of filtered Payment(s) and pages (of "Limit" # of items)
	TotalPayments int `json:"totalPayments,omitempty"`
	TotalPages    int `json:"totalPages,omitempty"`
	// Array of "limit" number of Payment(s)
	Payments []*Payment `json:"payments"`
}

// GetFilteredPaymentsFromStoreID is called by both PaymentsGetHandler (in this file) and PaymentsGetHandler (in webapp/store/handler.go)
func GetFilteredPaymentsFromStoreID(c *gin.Context, storeID int) {
	var (
		req  paymentsGetRequest
		resp paymentsGetResponse
	)

	// Get and Validate URL Query params
	err := c.ShouldBindQuery(&req)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			httperror.Send(c, http.StatusBadRequest, "Invalid query params")
			return
		}

		for _, err := range errs {
			httperror.Send(c, http.StatusUnprocessableEntity, paymentsGetFieldsErrors[err.Field()])
			return
		}
	}

	// Fill empty query params (necessary because default value in struct binding only accounts for unset params, not for params set to empty)
	if req.SortBy == "" {
		req.SortBy = "creation_time"
	}
	if req.OrderBy == "" {
		req.OrderBy = "desc"
	}

	resp.Limit = req.Limit
	resp.Page = req.Page

	var errCode int
	resp.Payments, resp.TotalPayments, resp.TotalPages, errCode, err = FetchFilteredPayments(storeID, req.Limit, req.Page, req.SortBy, req.OrderBy, req.Status, req.Currency)
	if err != nil {
		if errCode == http.StatusInternalServerError {
			httperror.Send500(c, err, "Error fetching filtered payments")
			return
		}

		httperror.Send(c, errCode, err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// PaymentsGetHandler handles GET requests to /api/v1/payments
func PaymentsGetHandler(c *gin.Context) {
	storeID := c.MustGet("storeID").(int)
	GetFilteredPaymentsFromStoreID(c, storeID)
}
