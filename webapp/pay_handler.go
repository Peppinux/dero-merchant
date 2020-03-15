package webapp

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/processor"
)

type paymentInfo struct {
	PaymentID         string
	Status            string
	Currency          string
	CurrencyAmount    float64
	ExchangeRate      float64
	DeroAmount        string
	IntegratedAddress string
	TTL               int
}

type payData struct {
	StoreTitle  string
	PaymentInfo *paymentInfo
}

func renderPay404(c *gin.Context) {
	c.HTML(http.StatusNotFound, "pay_404.html", nil)
}

func renderPay500(c *gin.Context, err error, description string) {
	c.HTML(http.StatusInternalServerError, "pay_500.html", nil)
	c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, description)) // Error is only logged and not sent to the user for security reasons
}

// PayHandler handles GET requests to /pay/:payment_id
func PayHandler(c *gin.Context) {
	var data payData

	p := &paymentInfo{
		PaymentID: c.Param("payment_id"),
	}

	var minsFromCreation int
	err := postgres.DB.QueryRow(`
		SELECT stores.title as store_title, payments.status, payments.currency, payments.currency_amount, payments.exchange_rate, payments.dero_amount, payments.integrated_address, CEIL(EXTRACT('epoch' FROM NOW() - payments.creation_time) / 60) as mins_from_creation
		FROM payments INNER JOIN stores ON payments.store_id=stores.id
		WHERE payments.payment_id=$1`, p.PaymentID).
		Scan(&data.StoreTitle, &p.Status, &p.Currency, &p.CurrencyAmount, &p.ExchangeRate, &p.DeroAmount, &p.IntegratedAddress, &minsFromCreation)
	if err != nil {
		if err == sql.ErrNoRows {
			renderPay404(c)
			return
		}

		renderPay500(c, err, "Error querying database")
		return
	}

	if p.Status == processor.PaymentStatusPending {
		p.TTL = config.PaymentMaxTTL - minsFromCreation
		if p.TTL < 0 {
			p.TTL = 0
		}
	}

	data.PaymentInfo = p
	c.HTML(http.StatusOK, "pay.html", data)
}

// WSPaymentStatusHandler handles GET requests to /ws/payment/:payment_id/status
func WSPaymentStatusHandler(c *gin.Context) {
	paymentID := c.Param("payment_id")

	var status string
	err := postgres.DB.QueryRow(`
		SELECT status
		FROM payments
		WHERE payment_id=$1`, paymentID).
		Scan(&status)
	if err != nil {
		log.Println("Error querying database:", err)
		return
	}

	if status != processor.PaymentStatusPending {
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		log.Println("Error upgrading HTTP:", err)
		return
	}

	processor.PaymentWSConnections[paymentID] = append(processor.PaymentWSConnections[paymentID], conn)
}
