package store

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/cryptoutil"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
)

// Store represents a store owned by a user
type Store struct {
	ID               int
	Title            string
	WalletViewKey    string
	Webhook          string
	WebhookSecretKey string
	APIKey           string
	SecretKey        string
	OwnerID          int
	Removed          bool
}

// HasValidTitle returns whether the title of Store has a valid length or not
func (s *Store) HasValidTitle() bool {
	if len(s.Title) > 0 && len(s.Title) <= 32 {
		return true
	}
	return false
}

// HasValidViewKey returns whether the Wallet View Key of Store has a valid length or not
func (s *Store) HasValidViewKey() bool {
	if len(s.WalletViewKey) == 128 {
		return true
	}
	return false
}

// HasUniqueTitle returns whther the title of Store is unique or not
func (s *Store) HasUniqueTitle() (bool, error) {
	lowerTitle := strings.ToLower(s.Title)

	var existingTitle string
	err := postgres.DB.QueryRow(`
		SELECT LOWER(title)
		FROM stores 
		WHERE owner_id=$1 AND LOWER(title)=$2 AND removed=$3`, s.OwnerID, lowerTitle, false).
		Scan(&existingTitle)
	if err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}

		return false, err
	}

	return false, nil
}

func generateStoreKeys() (apiKey, secretKey string, err error) {
	apiKey, err = stringutil.RandomHexString(32)
	if err != nil {
		err = errors.Wrap(err, "cannot generate random hex string")
		return
	}

	secretKey, err = stringutil.RandomHexString(32)
	if err != nil {
		err = errors.Wrap(err, "cannot generate random hex string")
	}

	return
}

func isUniqueAPIKey(apiKey string) (bool, error) {
	var storeID int
	err := postgres.DB.QueryRow(`
		SELECT id 
		FROM stores 
		WHERE api_key=$1`, apiKey).
		Scan(&storeID)
	if err != nil {
		if err == sql.ErrNoRows { // API Key is unique
			return true, nil
		}

		return false, errors.Wrap(err, "cannot query database")
	}

	return false, nil
}

// GenerateUniqueStoreKeys generates an API Key and a Secret Key that are not already in use by other stores
func GenerateUniqueStoreKeys() (apiKey, secretKey string, err error) {
	for {
		// Generate store keys
		apiKey, secretKey, err = generateStoreKeys()
		if err != nil {
			return "", "", errors.Wrap(err, "cannot generate store keys")
		}

		isUnique, err := isUniqueAPIKey(apiKey)
		if err != nil {
			return "", "", errors.Wrap(err, "cannot check if API Key is unique")
		}

		if isUnique {
			break
		}
	}

	return
}

func generateWebhookSecretKey() (secretKey string, err error) {
	secretKey, err = stringutil.RandomHexString(32)
	if err != nil {
		err = errors.Wrap(err, "cannot generate random hex string")
	}
	return
}

func isUniqueWebhookSecretKey(secretKey string) (bool, error) {
	var storeID int
	err := postgres.DB.QueryRow(`
			SELECT id
			FROM stores
			WHERE webhook_secret_key=$1`, secretKey).
		Scan(storeID)
	if err != nil {
		if err == sql.ErrNoRows { // Webhook Secret Key is unique
			return true, nil
		}

		return false, errors.Wrap(err, "cannot query database")
	}

	return false, nil
}

// GenerateUniqueWebhookSecretKey generates a Webhook Secret Key that is not already in use by other stores
func GenerateUniqueWebhookSecretKey() (secretKey string, err error) {
	for {
		// Generate Webhook Secret Key
		secretKey, err = generateWebhookSecretKey()
		if err != nil {
			return "", errors.Wrap(err, "cannot generate webhook secret key")
		}

		isUnique, err := isUniqueWebhookSecretKey(secretKey)
		if err != nil {
			return "", errors.Wrap(err, "cannot check if webhook secret key is unique")
		}

		if isUnique {
			break
		}
	}

	return
}

// CreateNewStore errors
var (
	ErrInvalidTitle   = errors.New("Invalid store Title")
	ErrInvalidViewKey = errors.New("Invalid store Wallet View Key")
	ErrTitleNotUnique = errors.New("Store Title not unique")
)

// CreateNewStore returns a new Store ready to be inserted into DB
func CreateNewStore(title, viewKey, webhook string, ownerID int) (s *Store, errs []error) {
	// Sanitize and validate input
	s = &Store{
		Title:         strings.TrimSpace(title),
		WalletViewKey: strings.TrimSpace(viewKey),
		Webhook:       strings.TrimSpace(webhook),
		OwnerID:       ownerID,
	}

	if !s.HasValidTitle() {
		errs = append(errs, ErrInvalidTitle)
	}
	if !s.HasValidViewKey() {
		errs = append(errs, ErrInvalidViewKey)
	}
	if errs != nil {
		return
	}

	// Check if new Store is unique
	unique, err := s.HasUniqueTitle()
	if err != nil {
		errs = append(errs, errors.Wrap(err, "cannot get if store has unique title"))
		return
	}

	if !unique {
		errs = append(errs, ErrTitleNotUnique)
		return
	}

	// Generate new Store Keys
	s.APIKey, s.SecretKey, err = GenerateUniqueStoreKeys()
	if err != nil {
		errs = append(errs, errors.Wrap(err, "cannot generate unique store keys"))
		return
	}

	// Generate Webhook Secret Key
	s.WebhookSecretKey, err = GenerateUniqueWebhookSecretKey()
	if err != nil {
		errs = append(errs, errors.Wrap(err, "cannot generate unique webhook secret key"))
		return
	}

	return
}

// Insert inserts a Store into DB
func (s *Store) Insert() error {
	err := postgres.DB.QueryRow(`
		INSERT INTO stores (title, wallet_view_key, webhook, webhook_secret_key, api_key, secret_key, owner_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`, s.Title, s.WalletViewKey, s.Webhook, s.WebhookSecretKey, s.APIKey, s.SecretKey, s.OwnerID).
		Scan(&s.ID)
	if err != nil {
		return errors.Wrap(err, "cannot query database")
	}

	// Save new Store ID and Title into Redis for future quick fetching from Dashboard
	redis.AddUserStore(s.OwnerID, s.ID)
	redis.SetStoreTitle(s.ID, s.Title)
	// Also save API Key data to make auth middleware faster
	redis.SetAPIKeyStore(s.APIKey, s.ID)
	redis.SetAPIKeySecretKey(s.APIKey, s.SecretKey)

	return nil
}

// FetchStoreFromID returns a Store fetched from DB based on its ID
func FetchStoreFromID(storeID, ownerID int) (s *Store, errCode int, err error) {
	s = &Store{
		ID:      storeID,
		OwnerID: ownerID,
	}

	err = postgres.DB.QueryRow(`
		SELECT title, wallet_view_key, webhook, webhook_secret_key, api_key, secret_key 
		FROM stores 
		WHERE id=$1 AND owner_id=$2 AND removed=$3`, s.ID, s.OwnerID, false).
		Scan(&s.Title, &s.WalletViewKey, &s.Webhook, &s.WebhookSecretKey, &s.APIKey, &s.SecretKey)
	if err != nil {
		if err == sql.ErrNoRows {
			errCode = http.StatusNotFound
		} else {
			errCode = http.StatusInternalServerError
			err = errors.Wrap(err, "cannot query database")
		}

		return
	}

	return
}

// UpdateViewKey errors
var (
	ErrForbidden             = errors.New("Forbidden")
	ErrInvalidViewKeyVerbose = errors.New("Wallet View Key needs to be either 128 characters long")
)

// UpdateViewKey updates Store's Wallet View Key in DB
func (s *Store) UpdateViewKey(newViewKey string) (errCode int, err error) {
	// Sanitize input
	s.WalletViewKey = strings.TrimSpace(newViewKey)

	// Validate input
	if !s.HasValidViewKey() {
		return http.StatusUnprocessableEntity, ErrInvalidViewKeyVerbose
	}

	// Update Store's Wallet View Key in DB
	res, err := postgres.DB.Exec(`
		UPDATE stores
		SET wallet_view_key=$1
		WHERE id=$2 AND owner_id=$3 AND removed=$4`, s.WalletViewKey, s.ID, s.OwnerID, false)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot execute query")
	}

	// If Store was not updated in DB, most likely because user had no permission to, return error
	numRows, _ := res.RowsAffected()
	if numRows == 0 {
		return http.StatusForbidden, ErrForbidden
	}

	return
}

// UpdateWebhook updates Stores' Webhook URL in DB
func (s *Store) UpdateWebhook(newWebhook string) (errCode int, err error) {
	// Sanitize input
	s.Webhook = strings.TrimSpace(newWebhook)

	// Update Store's Webhook URL in DB
	res, err := postgres.DB.Exec(`
		UPDATE stores
		SET webhook=$1
		WHERE id=$2 AND owner_id=$3 AND removed=$4`, s.Webhook, s.ID, s.OwnerID, false)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot execute query")
	}

	// If Store was not updated in DB, most likely because user had no permission to, return error
	numRows, _ := res.RowsAffected()
	if numRows == 0 {
		return http.StatusForbidden, ErrForbidden
	}

	return
}

// UpdateWebhookSecretKey generates a new Webhook Secret Key for Store and updates it in DB
func (s *Store) UpdateWebhookSecretKey() (errCode int, err error) {
	s.WebhookSecretKey, err = GenerateUniqueWebhookSecretKey()
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot generate unique webhook secret key")
	}

	// Update Store in DB (Set new Webhook Secret Key)
	res, err := postgres.DB.Exec(`
		UPDATE stores
		SET webhook_secret_key=$1
		WHERE id=$2 AND owner_id=$3 AND removed=$4`, s.WebhookSecretKey, s.ID, s.OwnerID, false)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot execute query")
	}

	// If Store was not updated in DB, most likely because user had no permission to, return error
	numRows, _ := res.RowsAffected()
	if numRows == 0 {
		return http.StatusForbidden, ErrForbidden
	}

	return
}

// UpdateKeys generates new API and Secret Key for Store and updates it in DB
func (s *Store) UpdateKeys() (errCode int, err error) {
	// Fetch current API Key in order to remove it from Redis
	err = postgres.DB.QueryRow(`
		SELECT api_key 
		FROM stores 
		WHERE id=$1 AND owner_id=$2 AND removed=$3`, s.ID, s.OwnerID, false).
		Scan(&s.APIKey)
	if err != nil {
		if err == sql.ErrNoRows {
			// If Store was not found in DB, most likely because user had no permission to, return error
			return http.StatusForbidden, ErrForbidden
		}

		return http.StatusInternalServerError, errors.Wrap(err, "cannot query database")
	}

	redis.DeleteAPIKeyStore(s.APIKey)
	redis.DeleteAPIKeySecretKey(s.APIKey)

	// Generate new API Key and Secret Key
	s.APIKey, s.SecretKey, err = GenerateUniqueStoreKeys()
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot generate unique store keys")
	}

	// Update Store's API and Secret Key in DB
	res, err := postgres.DB.Exec(`
		UPDATE stores
		SET api_key=$1, secret_key=$2
		WHERE id=$3 AND owner_id=$4 AND removed=$5`, s.APIKey, s.SecretKey, s.ID, s.OwnerID, false)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot execute query")
	}

	// If Store was not updated in DB, most likely because user had no permission to, return error
	numRows, _ := res.RowsAffected()
	if numRows == 0 {
		return http.StatusForbidden, ErrForbidden
	}

	// Save new API Key and Secret Key in Redis
	redis.SetAPIKeyStore(s.APIKey, s.ID)
	redis.SetAPIKeySecretKey(s.APIKey, s.SecretKey)

	return
}

// Remove removes a Store from DB
func (s *Store) Remove() (errCode int, err error) {
	// Update Store in DB (Set removed=true)
	err = postgres.DB.QueryRow(`
		UPDATE stores 
		SET removed=$1 
		WHERE id=$2 AND owner_id=$3 AND removed=$4
		RETURNING api_key`, true, s.ID, s.OwnerID, false).
		Scan(&s.APIKey)
	if err != nil {
		if err == sql.ErrNoRows {
			// If Store was not updated in DB, most likely because user had no permission to, return error
			return http.StatusForbidden, ErrForbidden
		}

		return http.StatusInternalServerError, errors.Wrap(err, "cannot execute query")
	}

	// Remove Store ID, Title and Owner from Redis
	redis.RemoveUserStore(s.OwnerID, s.ID)
	redis.DeleteStoreTitle(s.ID)
	// Remove Store ID and Secret Key associated to Hashed API Key in Redis
	hashedAPIKey := cryptoutil.HashStringToSHA256Hex(s.APIKey)
	redis.DeleteAPIKeyStore(hashedAPIKey)
	redis.DeleteAPIKeySecretKey(hashedAPIKey)

	return
}
