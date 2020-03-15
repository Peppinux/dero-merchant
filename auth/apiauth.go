package auth

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/peppinux/dero-merchant/cryptoutil"
	"github.com/peppinux/dero-merchant/httperror"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/redis"
)

// APIKeyAuth provides a middleware that rejects unauthenticated requests to the API
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get and validate X-API-Key header
		apiKey := c.GetHeader("X-API-Key")
		if len(apiKey) != 64 {
			httperror.Send(c, http.StatusBadRequest, "Invalid Header X-API-Key: required 64 characters long string")
			return
		}

		// Fetch Store ID associated to (hashed) API Key from Redis
		hashedAPIKey := cryptoutil.HashStringToSHA256Hex(apiKey)
		storeID, err := redis.GetAPIKeyStore(hashedAPIKey)
		if err != nil {
			// If Store ID was not found in Redis, try fetching it from DB
			err := postgres.DB.QueryRow(`
				SELECT id
				FROM stores
				WHERE api_key=$1 AND removed=$2`, apiKey, false).
				Scan(&storeID)
			if err != nil {
				if err == sql.ErrNoRows { // No store associated to API Key was found
					httperror.Send(c, http.StatusForbidden, "Invalid API Key")
				} else {
					httperror.Send500(c, err, "Error querying database")
				}

				return
			}

			// Store value in Redis for quick retrieving in future requests
			redis.SetAPIKeyStore(hashedAPIKey, storeID)
		}

		c.Set("apiKey", apiKey)
		c.Set("storeID", storeID)
		c.Next()
	}
}

// SecretKeyAuth provides a middleware that rejects unauthorized requests to the API. Needs to be used in conjuction with APIKeyAuth
func SecretKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get and validate X-Signature header
		signature := c.GetHeader("X-Signature")
		if len(signature) != 64 {
			httperror.Send(c, http.StatusBadRequest, "Invalid Header X-Signature: required 64 characters long SHA256 hex encoded string")
			return
		}

		apiKey := c.MustGet("apiKey").(string)
		hashedAPIKey := cryptoutil.HashStringToSHA256Hex(apiKey)

		// Fetch Secret Key associated to API Key from Redis
		secretKey, err := redis.GetAPIKeySecretKey(hashedAPIKey)
		if err != nil {
			// If Secret Key was not found in Redis, try fetching it from DB
			err = postgres.DB.QueryRow(`
				SELECT secret_key
				FROM stores
				WHERE api_key=$1 AND removed=$2`, apiKey, false).
				Scan(&secretKey)
			if err != nil {
				if err == sql.ErrNoRows {
					httperror.Send(c, http.StatusUnauthorized, "Invalid Signature")
				} else {
					httperror.Send500(c, err, "Error querying database")
				}

				return
			}

			// Store value in Redis for quick retrieving in future requests
			redis.SetAPIKeySecretKey(hashedAPIKey, secretKey)
		}

		body, _ := c.GetRawData()                                // Read request body from stream
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body)) // Copy body back into stream for consumption by next route

		signatureBytes, err := hex.DecodeString(signature)
		if httperror.Send500IfErr(c, err, "Error decoding hex string") != nil {
			return
		}

		secretKeyBytes, err := hex.DecodeString(secretKey)
		if httperror.Send500IfErr(c, err, "Error decoding hex string") != nil {
			return
		}

		// Verify Signature
		validSignature, err := cryptoutil.ValidMAC(body, signatureBytes, secretKeyBytes)
		if httperror.Send500IfErr(c, err, "Error verifying signature") != nil {
			return
		}

		if !validSignature {
			httperror.Send(c, http.StatusUnauthorized, "Invalid Signature")
			return
		}

		c.Next()
	}
}
