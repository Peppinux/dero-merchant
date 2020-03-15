package auth

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/cryptoutil"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
)

type APIAuthUserMock struct {
	ID                int
	Username          string
	Email             string
	Password          string
	HashedPassword    string
	VerificationToken string
	Verified          bool
}

type APIAuthStoreMock struct {
	ID               int
	Title            string
	ViewKey          string
	Webhook          string
	WebhookSecretKey string
	APIKey           string
	SecretKey        string
	OwnerID          int
}

type APIAuthTestSuite struct {
	suite.Suite

	mockUser  *APIAuthUserMock
	mockStore *APIAuthStoreMock

	doRequest func(r *gin.Engine, body, apiKey, secretKey string) *httptest.ResponseRecorder
}

func (suite *APIAuthTestSuite) SetupSuite() {
	err := config.LoadFromENV("../.env")
	if err != nil {
		panic(err)
	}

	redis.Pool = redis.NewPool(config.TestRedisAddress)
	err = redis.Ping()
	if err != nil {
		panic(err)
	}

	err = redis.FlushAll()
	if err != nil {
		panic(err)
	}

	postgres.DB, err = postgres.Connect(config.TestDBName, config.TestDBUser, config.TestDBPassword, config.TestDBHost, config.TestDBPort, "disable") // TODO: Enable SSLMode?
	if err != nil {
		panic(err)
	}

	postgres.DropTables()
	postgres.CreateTablesIfNotExist()

	suite.mockUser = &APIAuthUserMock{
		Username: "Test user foo",
		Email:    "foo@bar.baz",
		Password: "foobarbaz",
		Verified: true,
	}

	suite.mockStore = &APIAuthStoreMock{
		Title:   "Test store bar",
		ViewKey: "foobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbazfoobarbaz12",
		Webhook: "",
	}

	u := suite.mockUser
	s := suite.mockStore

	u.HashedPassword, err = argon2id.CreateHash(u.Password, argon2id.DefaultParams)
	if err != nil {
		panic(err)
	}

	u.VerificationToken, err = stringutil.RandomBase64RawURLString(48)
	if err != nil {
		panic(err)
	}

	err = postgres.DB.QueryRow(`
		INSERT INTO users (username, email, password, verification_token, email_verified)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, u.Username, u.Email, u.HashedPassword, u.VerificationToken, u.Verified).
		Scan(&u.ID)
	if err != nil {
		panic(err)
	}

	s.OwnerID = u.ID

	s.WebhookSecretKey, err = stringutil.RandomHexString(32)
	if err != nil {
		panic(err)
	}

	s.APIKey, err = stringutil.RandomHexString(32)
	if err != nil {
		panic(err)
	}

	s.SecretKey, err = stringutil.RandomHexString(32)
	if err != nil {
		panic(err)
	}

	err = postgres.DB.QueryRow(`
		INSERT INTO stores (title, wallet_view_key, webhook, webhook_secret_key, api_key, secret_key, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`, s.Title, s.ViewKey, s.Webhook, s.WebhookSecretKey, s.APIKey, s.SecretKey, s.OwnerID).
		Scan(&s.ID)
	if err != nil {
		panic(err)
	}

	// HTTP request reusable function
	suite.doRequest = func(r *gin.Engine, body, apiKey, secretKey string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/", strings.NewReader(body))
		if apiKey != "" {
			req.Header.Add("X-API-Key", apiKey)
		}
		if secretKey != "" {
			key, _ := hex.DecodeString(secretKey)
			sign, _ := cryptoutil.SignMessage([]byte(body), key)
			hexSign := hex.EncodeToString(sign)
			req.Header.Add("X-Signature", hexSign)
		}
		r.ServeHTTP(w, req)
		return w
	}
}

func (suite *APIAuthTestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()

	postgres.DropTables()
	postgres.DB.Close()
}

func TestAPIAuthTestSuite(t *testing.T) {
	suite.Run(t, new(APIAuthTestSuite))
}

func (suite *APIAuthTestSuite) TestAPIKeyAuth() {
	s := suite.mockStore

	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(APIKeyAuth())

	r.POST("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Valid API Key
	w := suite.doRequest(r, "", s.APIKey, "")

	suite.Equal(http.StatusOK, w.Code)

	// Valid API Key again in order to fetch it from Redis instead of Postgres
	w = suite.doRequest(r, "", s.APIKey, "")

	suite.Equal(http.StatusOK, w.Code)

	// No API Key
	w = suite.doRequest(r, "", "", "")

	suite.Equal(http.StatusBadRequest, w.Code)

	// Inexistent API Key
	k, _ := stringutil.RandomHexString(32)
	w = suite.doRequest(r, "", k, "")

	suite.Equal(http.StatusForbidden, w.Code)
}

func (suite *APIAuthTestSuite) TestSecretKeyAuth() {
	s := suite.mockStore

	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(APIKeyAuth())
	r.Use(SecretKeyAuth())

	r.POST("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	testBody := `{"foo":"bar"}`

	// Valid Secret Key
	w := suite.doRequest(r, testBody, s.APIKey, s.SecretKey)

	suite.Equal(http.StatusOK, w.Code)

	// No Secret Key
	w = suite.doRequest(r, testBody, s.APIKey, "")

	suite.Equal(http.StatusBadRequest, w.Code)

	// Invalid Secret Key
	k, _ := stringutil.RandomHexString(32)
	w = suite.doRequest(r, testBody, s.APIKey, k)

	suite.Equal(http.StatusUnauthorized, w.Code)
}
