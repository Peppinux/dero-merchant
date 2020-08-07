package processor

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/cryptoutil"
)

// Webhook is a type that contains the URL and Secret Key of the Webhook of a store, and whose main purpose is to send events to said URL
type Webhook struct {
	URL       string
	SecretKey string
}

// PaymentUpdateEvent is the event sent to the Webhook URL when the status of a payment changes
type PaymentUpdateEvent struct {
	PaymentID string `json:"paymentID,omitempty"`
	Status    string `json:"status,omitempty"`
}

// IsSet returns whether valid Webhook URL and Secret Key are set in the struct
func (w *Webhook) IsSet() bool {
	return w.URL != "" && w.SecretKey != ""
}

// SendPaymentUpdateEvent sends a signed PaymentUpdateEvent to the Webhook URL
func (w *Webhook) SendPaymentUpdateEvent(paymentID string, newStatus string) error {
	e := &PaymentUpdateEvent{
		PaymentID: paymentID,
		Status:    newStatus,
	}

	body, err := json.Marshal(e)
	if err != nil {
		return errors.Wrap(err, "cannot marshal event")
	}

	secretKeyBytes, err := hex.DecodeString(w.SecretKey)
	if err != nil {
		return errors.Wrap(err, "cannot decode hex string")
	}

	bodySignature, err := cryptoutil.SignMessage(body, secretKeyBytes)
	if err != nil {
		return errors.Wrap(err, "cannot sign message")
	}

	bodySignatureHex := hex.EncodeToString(bodySignature)

	req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrap(err, "cannot create new request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", bodySignatureHex)

	httpClient := &http.Client{
		Timeout: time.Second, // Don't need a response
	}
	_, err = httpClient.Do(req)

	return nil
}
