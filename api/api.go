package api

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"

	deroglobals "github.com/deroproject/derosuite/globals"

	"github.com/peppinux/dero-merchant/coingecko"
	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/processor"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
)

// Payment represents a payment made to a store
type Payment struct {
	PaymentID         string    `json:"paymentID,omitempty"`
	Status            string    `json:"status,omitempty"`
	Currency          string    `json:"currency,omitempty"`
	CurrencyAmount    float64   `json:"currencyAmount,omitempty"`
	ExchangeRate      float64   `json:"exchangeRate,omitempty"`
	DeroAmount        string    `json:"deroAmount,omitempty"`
	AtomicDeroAmount  uint64    `json:"atomicDeroAmount,omitempty"`
	IntegratedAddress string    `json:"integratedAddress,omitempty"`
	CreationTime      time.Time `json:"creationTime,omitempty"`
	TTL               int       `json:"ttl"`
	StoreID           int       `json:"-"`
}

// HasValidCurrency returns whether the currency of Payment is supported by CoinGecko API or not
func (p *Payment) HasValidCurrency() bool {
	currency := strings.ToLower(p.Currency)

	if currency == "dero" {
		return true
	}

	// Check if currency is in cached set of supported currencies in Redis
	supported, _ := redis.IsSupportedCurrency(currency)
	if supported {
		return true
	}

	// If currency is not in cached set, get supported currencies from CoinGecko API
	currencies, err := coingecko.SupportedVsCurrencies()
	if err != nil {
		return false
	}

	// Update set in Redis
	go redis.SetSupportedCurrencies(currencies)

	// Check if currency is supported
	for _, c := range currencies {
		if currency == c {
			return true
		}
	}

	return false
}

// HasValidCurrencyAmount checks if the amount of currency of Payment is a positive number
func (p *Payment) HasValidCurrencyAmount() bool {
	return p.CurrencyAmount > 0
}

func isUniqueIntegratedAddress(iaddr, payid string) (bool, error) {
	var existingPaymentID string
	err := postgres.DB.QueryRow(`
			SELECT payment_id
			FROM payments
			WHERE payment_id=$1 OR integrated_address=$2`, payid, iaddr).
		Scan(&existingPaymentID)
	if err != nil {
		if err == sql.ErrNoRows { // Integrated Address and PaymentID are unique
			return true, nil
		}

		return false, errors.Wrap(err, "cannot query database")
	}

	return false, nil
}

// GenerateUniqueIntegratedAddress returns an integrated address and its payment ID, that have never been used for any other payments before
func GenerateUniqueIntegratedAddress(w *processor.StoreWallet) (iaddr, payid string, err error) {
	for {
		iaddr, payid = w.GenerateIntegratedAddress()

		isUnique, err := isUniqueIntegratedAddress(iaddr, payid)
		if err != nil {
			return "", "", errors.Wrap(err, "cannot check if generated integrated address is unique")
		}

		if isUnique {
			break
		}
	}

	return
}

// CalculateTTL calculates and updates Payment TTL based on the number of minutes passed from the creation of the payment
func (p *Payment) CalculateTTL(minsFromCreation int) {
	if p.Status == processor.PaymentStatusPending {
		p.TTL = config.PaymentMaxTTL - minsFromCreation
		if p.TTL < 0 {
			p.TTL = 0
		}
	}
}

// CreateNewPayment errors
var (
	ErrInvalidCurrency = errors.New("Invalid Param 'currency': required 3-4 chars long string")
	ErrInvalidAmount   = errors.New("Invalid Param 'amount': required .12f float")
)

// CreateNewPayment returns a new Payment ready to be stored in DB and be listened to by processor
func CreateNewPayment(currency string, currencyAmount float64, storeID int) (p *Payment, w *processor.StoreWallet, errCode int, err error) {
	p = &Payment{
		Status:  processor.PaymentStatusPending,
		TTL:     config.PaymentMaxTTL,
		StoreID: storeID,
	}

	// Validate params
	p.Currency = strings.ToUpper(currency)
	if !p.HasValidCurrency() {
		return nil, nil, http.StatusUnprocessableEntity, ErrInvalidCurrency
	}
	p.CurrencyAmount = currencyAmount
	if !p.HasValidCurrencyAmount() {
		return nil, nil, http.StatusUnprocessableEntity, ErrInvalidAmount
	}

	if p.Currency == "DERO" {
		p.ExchangeRate = 1
		p.DeroAmount = fmt.Sprintf("%.12f", p.CurrencyAmount)
	} else {
		// Get current exchange rate from CoinGecko API
		exchangeRate, err := coingecko.DeroPrice(p.Currency) // DERO value in payment currency. 1 DERO = x CURRENCY. Exchange Rate = x CURRENCY
		if err != nil {
			return nil, nil, http.StatusInternalServerError, errors.Wrap(err, "cannot get DERO price")
		}

		// Convert amount of currency to DERO
		p.ExchangeRate = exchangeRate
		deroAmount := p.CurrencyAmount / exchangeRate // 1 DERO : Exchange Rate = Dero Amount : Currency Amount => Dero Amount = 1 * Currency Amount / Exchange Rate
		p.DeroAmount = fmt.Sprintf("%.12f", deroAmount)
	}

	// Convert amount of DERO to atomic DERO
	p.AtomicDeroAmount, err = deroglobals.ParseAmount(p.DeroAmount)
	if err != nil {
		return nil, nil, http.StatusUnprocessableEntity, ErrInvalidAmount
	}

	w, err = processor.ActiveWallets.GetWalletFromStoreID(p.StoreID)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, errors.Wrap(err, "cannot get wallet from Store ID")
	}

	err = w.DeroWallet.IsDaemonOnline()
	if err != nil {
		return nil, nil, http.StatusInternalServerError, errors.Wrap(err, "daemon offline")
	}

	p.IntegratedAddress, p.PaymentID, err = GenerateUniqueIntegratedAddress(w)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, errors.Wrap(err, "cannot generate unique integrated address")
	}

	return
}

// Insert inserts a Payment into DB
func (p *Payment) Insert() error {
	err := postgres.DB.QueryRow(`
		INSERT INTO payments (payment_id, status, currency, currency_amount, exchange_rate, dero_amount, atomic_dero_amount, integrated_address, store_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9) 
		RETURNING creation_time`, p.PaymentID, p.Status, p.Currency, p.CurrencyAmount, p.ExchangeRate, p.DeroAmount, p.AtomicDeroAmount, p.IntegratedAddress, p.StoreID).
		Scan(&p.CreationTime)
	if err != nil {
		return errors.Wrap(err, "cannot query database")
	}

	return nil
}

// Payment(s) not found errors
var (
	ErrPaymentNotFound     = errors.New("Payment not found")
	ErrPaymentsNotFound    = errors.New("Payments not found")
	ErrNoPaymentsFound     = errors.New("No payments found")
	ErrNoPaymentsFoundPage = errors.New("No payments found on this page")
)

// FetchPaymentFromID returns a Payment fetched from DB based on its Payment ID
func FetchPaymentFromID(paymentID string, storeID int) (p *Payment, errCode int, err error) {
	p = &Payment{
		PaymentID: paymentID,
		StoreID:   storeID,
	}

	var minsFromCreation int
	err = postgres.DB.QueryRow(`
		SELECT status, currency, currency_amount, exchange_rate, dero_amount, atomic_dero_amount, integrated_address, creation_time, CEIL(EXTRACT('epoch' FROM NOW() - creation_time) / 60) 
		FROM payments 
		WHERE payment_id=$1 AND store_id=$2`, p.PaymentID, p.StoreID).
		Scan(&p.Status, &p.Currency, &p.CurrencyAmount, &p.ExchangeRate, &p.DeroAmount, &p.AtomicDeroAmount, &p.IntegratedAddress, &p.CreationTime, &minsFromCreation)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound, ErrPaymentNotFound
		}

		return nil, http.StatusInternalServerError, errors.Wrap(err, "cannot query database")
	}

	p.CalculateTTL(minsFromCreation)

	return
}

// FetchPaymentsFromIDs returns a slice of Payments fetched from DB based on their Payment IDs
func FetchPaymentsFromIDs(paymentIDs []string, storeID int) (ps []*Payment, errCode int, err error) {
	rows, err := postgres.DB.Query(`
		SELECT payment_id, status, currency, currency_amount, exchange_rate, dero_amount, atomic_dero_amount, integrated_address, creation_time, CEIL(EXTRACT('epoch' FROM NOW() - creation_time) / 60) 
		FROM payments
		WHERE store_id=$1 AND payment_id = ANY($2)`, storeID, pq.Array(paymentIDs))
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "cannot query database")
	}

	defer rows.Close()

	var minsFromCreation int
	for rows.Next() {
		var p Payment
		err := rows.Scan(&p.PaymentID, &p.Status, &p.Currency, &p.CurrencyAmount, &p.ExchangeRate, &p.DeroAmount, &p.AtomicDeroAmount, &p.IntegratedAddress, &p.CreationTime, &minsFromCreation)
		if err != nil {
			continue
		}

		p.CalculateTTL(minsFromCreation)

		ps = append(ps, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "cannot iterate over rows")
	}

	if len(ps) == 0 {
		return nil, http.StatusNotFound, ErrPaymentsNotFound
	}

	return
}

// FetchFilteredPayments returns a slice of Payments fetched from DB based on given filters
func FetchFilteredPayments(storeID, limit, page int, sortBy, orderBy, statusFilter, currencyFilter string) (ps []*Payment, totalPayments, totalPages, errCode int, err error) {
	// Note: Input comes already sanitized from caller function GetPaymentsFromStoreID.

	// Fetch total number of filtered payments from DB
	err = postgres.DB.QueryRow(`
		SELECT COUNT(*)
		FROM payments
		WHERE store_id=$1 AND ($2='' OR status=LOWER($2)) AND ($3='' OR currency=UPPER($3))`, storeID, statusFilter, currencyFilter).
		Scan(&totalPayments)
	if err != nil {
		errCode = http.StatusInternalServerError
		err = errors.Wrap(err, "cannot query database")
		return
	}

	if totalPayments == 0 {
		errCode = http.StatusNotFound
		err = ErrNoPaymentsFound
		return
	}

	baseQuery := `
		SELECT payment_id, status, currency, currency_amount, exchange_rate, dero_amount, atomic_dero_amount, integrated_address, creation_time, CEIL(EXTRACT('epoch' FROM NOW() - creation_time) / 60) 
		FROM payments 
		WHERE store_id=$1 AND ($2='' OR status=LOWER($2)) AND ($3='' OR currency=UPPER($3)) 
	`
	orderByQuery := fmt.Sprintf(`ORDER BY %s %s `, sortBy, orderBy) // SQL Injection safe because params were previously validated. Could not use named parameters.
	limitQuery := ""

	if limit > 0 {
		offset := (page - 1) * limit
		limitQuery = fmt.Sprintf(`LIMIT %d OFFSET %d `, limit, offset)

		totalPages = int(math.Ceil(float64(totalPayments) / float64(limit)))
	} else {
		totalPages = 1
	}

	if page > totalPages {
		errCode = http.StatusNotFound
		err = ErrNoPaymentsFoundPage
		return
	}

	// Fetch filtered payments from DB
	query := stringutil.Build(baseQuery, orderByQuery, limitQuery)
	rows, err := postgres.DB.Query(query, storeID, statusFilter, currencyFilter)
	if err != nil {
		errCode = http.StatusInternalServerError
		err = errors.Wrap(err, "cannot query database")
		return
	}

	defer rows.Close()

	var minsFromCreation int
	for rows.Next() {
		var p Payment
		err = rows.Scan(&p.PaymentID, &p.Status, &p.Currency, &p.CurrencyAmount, &p.ExchangeRate, &p.DeroAmount, &p.AtomicDeroAmount, &p.IntegratedAddress, &p.CreationTime, &minsFromCreation)
		if err != nil {
			errCode = http.StatusInternalServerError
			err = errors.Wrap(err, "cannot scan row")
			return
		}

		p.CalculateTTL(minsFromCreation)

		ps = append(ps, &p)
	}

	if err = rows.Err(); err != nil {
		errCode = http.StatusInternalServerError
		err = errors.Wrap(err, "cannot iterate over rows")
		return
	}

	return
}
