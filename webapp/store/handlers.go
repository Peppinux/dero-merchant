package store

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/peppinux/dero-merchant/api"
	"github.com/peppinux/dero-merchant/auth"
	"github.com/peppinux/dero-merchant/httperror"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/processor"
	"github.com/peppinux/dero-merchant/redis"
)

type storePutRequest struct {
	ViewKey             *string `json:"viewKey"`
	Webhook             *string `json:"webhook"`
	NewWebhookSecretKey bool    `json:"newWebhookSecretKey"`
	NewStoreKeys        bool    `json:"newStoreKeys"`
}

type storePutResponse struct {
	ViewKey          string `json:"viewKey,omitempty"`
	Webhook          string `json:"webhook,omitempty"`
	WebhookSecretKey string `json:"webhookSecretKey,omitempty"`
	APIKey           string `json:"apiKey,omitempty"`
	SecretKey        string `json:"secretKey,omitempty"`
}

// PutHandler handles PUT requests to /store/:id
func PutHandler(c *gin.Context) {
	// Get Store ID from URL Params
	storeID, err := strconv.Atoi(c.Param("id"))
	if httperror.Send500IfErr(c, err, "Error converting string to int") != nil {
		return
	}

	// Get User data from session
	s := c.MustGet("session").(*auth.Session)

	store := &Store{
		ID:      storeID,
		OwnerID: s.UserID,
	}

	var (
		req  storePutRequest
		resp storePutResponse
	)

	// Get input submitted by user through JSON
	err = c.ShouldBindJSON(&req)
	if httperror.Send500IfErr(c, err, "Error binding JSON to fields") != nil {
		return
	}

	switch {
	case req.ViewKey != nil: // Edit Wallet View Key
		// Check if wallet associated to current View Key is waiting for payments.
		// If it is, View Key cannot be modified, therefore send error message.
		if processor.ActiveWallets.HasWalletFromStoreID(store.ID) {
			w, err := processor.ActiveWallets.GetWalletFromStoreID(store.ID)
			if httperror.Send500IfErr(c, err, "Error getting wallet from store ID") != nil {
				return
			}

			if w.PendingPayments.Count() > 0 {
				httperror.Send(c, http.StatusForbidden, "Wallet associated to current View Key is still waiting for pending payments")
				return
			}
		}

		errCode, err := store.UpdateViewKey(*req.ViewKey)
		if err != nil {
			if errCode == http.StatusInternalServerError {
				httperror.Send500(c, err, "Error updating store's wallet view key")
				return
			}

			httperror.Send(c, errCode, err.Error())
			return
		}

		resp.ViewKey = store.WalletViewKey
		c.JSON(http.StatusOK, resp)

	case req.Webhook != nil: // Edit Webhook
		errCode, err := store.UpdateWebhook(*req.Webhook)
		if err != nil {
			if errCode == http.StatusInternalServerError {
				httperror.Send500(c, err, "Error updating store's webhook")
				return
			}

			httperror.Send(c, errCode, err.Error())
			return
		}

		// Check if wallet associated to store is waiting for payments.
		// If it is, update webhook url of store wallet.
		if processor.ActiveWallets.HasWalletFromStoreID(store.ID) {
			w, err := processor.ActiveWallets.GetWalletFromStoreID(store.ID)
			if httperror.Send500IfErr(c, err, "Error getting wallet from store ID") != nil {
				return
			}

			w.Webhook.URL = store.Webhook
		}

		resp.Webhook = store.Webhook
		c.JSON(http.StatusOK, resp)

	case req.NewWebhookSecretKey == true: // Generate new Webhook Secret Key
		errCode, err := store.UpdateWebhookSecretKey()
		if err != nil {
			if errCode == http.StatusInternalServerError {
				httperror.Send500(c, err, "Error updating store's webhook secret key")
				return
			}

			httperror.Send(c, errCode, err.Error())
			return
		}

		// Check if wallet associated to store is waiting for payments.
		// If it is, update webhook secret key of store wallet.
		if processor.ActiveWallets.HasWalletFromStoreID(store.ID) {
			w, err := processor.ActiveWallets.GetWalletFromStoreID(store.ID)
			if httperror.Send500IfErr(c, err, "Error getting wallet from store ID") != nil {
				return
			}

			w.Webhook.SecretKey = store.WebhookSecretKey
		}

		resp.WebhookSecretKey = store.WebhookSecretKey
		c.JSON(http.StatusOK, resp)

	case req.NewStoreKeys == true: // Generate new Store keys
		errCode, err := store.UpdateKeys()
		if err != nil {
			if errCode == http.StatusInternalServerError {
				httperror.Send500(c, err, "Error updating store's API and Secret key")
				return
			}

			httperror.Send(c, errCode, err.Error())
			return
		}

		resp.APIKey = store.APIKey
		resp.SecretKey = store.SecretKey
		c.JSON(http.StatusOK, resp)

	default: // Invalid request
		httperror.Send(c, http.StatusBadRequest, "Bad request")
	}
}

// DeleteHandler handles DELTE requests to /store/:id
func DeleteHandler(c *gin.Context) {
	// Get Store ID from URL Params
	storeID, err := strconv.Atoi(c.Param("id"))
	if httperror.Send500IfErr(c, err, "Error converting string to int") != nil {
		return
	}

	// Get User data from session
	s := c.MustGet("session").(*auth.Session)

	store := &Store{
		ID:      storeID,
		OwnerID: s.UserID,
	}

	// Check if wallet associated to store is waiting for payments.
	// If it is, store cannot be deleted, therefore send error message.
	if processor.ActiveWallets.HasWalletFromStoreID(store.ID) {
		w, err := processor.ActiveWallets.GetWalletFromStoreID(store.ID)
		if httperror.Send500IfErr(c, err, "Error getting wallet from store ID") != nil {
			return
		}

		if w.PendingPayments.Count() > 0 {
			httperror.Send(c, http.StatusForbidden, "Wallet associated to store is still waiting for pending payments")
			return
		}
	}

	errCode, err := store.Remove()
	if err != nil {
		if errCode == http.StatusInternalServerError {
			httperror.Send500(c, err, "Error removing store")
			return
		}

		httperror.Send(c, errCode, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// PaymentsGetHandler handles GET requests to /store/:id/payments
func PaymentsGetHandler(c *gin.Context) {
	// Get Store ID from URL Params
	storeID, err := strconv.Atoi(c.Param("id"))
	if httperror.Send500IfErr(c, err, "Error converting string to int") != nil {
		return
	}

	// Get User data from session
	s := c.MustGet("session").(*auth.Session)

	// Check if the user that made the request actually owns the store. If not, send error
	// Check in Redis first
	userOwnsStore, _ := redis.UserOwnsStore(s.UserID, storeID)
	if !userOwnsStore {
		// Fallback to DB if fetching from Redis failed
		err = postgres.DB.QueryRow(`
		SELECT (owner_id=$1)
		FROM stores
		WHERE id=$2 AND removed=$3`, s.UserID, storeID, false).
			Scan(&userOwnsStore)
		if httperror.Send500IfErr(c, err, "Error scanning row") != nil {
			return
		}
	}

	if !userOwnsStore {
		httperror.Send(c, http.StatusForbidden, "Forbidden")
		return
	}

	api.GetFilteredPaymentsFromStoreID(c, storeID)
}
