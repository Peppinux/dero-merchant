package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/stretchr/testify/suite"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/processor"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
)

type UserMock struct {
	ID                int
	Username          string
	Email             string
	Password          string
	HashedPassword    string
	VerificationToken string
	Verified          bool
}

type StoreMock struct {
	ID               int
	Title            string
	ViewKey          string
	Webhook          string
	WebhookSecretKey string
	APIKey           string
	SecretKey        string
	OwnerID          int
}

type APITestSuite struct {
	suite.Suite

	mockUser  *UserMock
	mockStore *StoreMock
}

func (suite *APITestSuite) SetupSuite() {
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

	suite.mockUser = &UserMock{
		Username: "Test user",
		Email:    "test@user.com",
		Password: "foobarbaz",
		Verified: true,
	}

	suite.mockStore = &StoreMock{
		Title:   "Test store",
		ViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c",
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

	config.DeroNetwork = config.TestDeroNetwork
	config.DeroDaemonAddress = config.TestDeroDaemonAddress
	processor.ActiveWallets = processor.NewStoresWallets()
	err = processor.SetupDaemonConnection()
	if err != nil {
		panic(err)
	}

	config.WalletsPath = config.TestWalletsPath
	err = processor.CreateWalletsDirectory()
	if err != nil {
		panic(err)
	}
}

func (suite *APITestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()

	postgres.DropTables()
	postgres.DB.Close()

	os.RemoveAll(config.TestWalletsPath)
}

func TestAPITestSuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}

func (suite *APITestSuite) TestHasValidCurrency() {
	testPayments := map[*Payment]bool{
		&Payment{Currency: "DERO"}: true,
		&Payment{Currency: "usd"}:  true,
		&Payment{Currency: "Eur"}:  true,
		&Payment{Currency: "ABC"}:  false,
		&Payment{Currency: "xyz"}:  false,
	}

	for p, shouldValid := range testPayments {
		isValid := p.HasValidCurrency()
		suite.Equal(shouldValid, isValid)

		// Test is so quick the goroutine the caches currencies in Redis does not finish execution.
		// This is the reason why line 50 of api.go is not cover.
		// To cover it in spite of testing speed uncomment the following line:
		// time.Sleep(time.Millisecond * 100)
	}
}

func (suite *APITestSuite) TestHasValidCurrencyAmount() {
	testPayments := map[*Payment]bool{
		&Payment{CurrencyAmount: 1}:        true,
		&Payment{CurrencyAmount: 0.1}:      true,
		&Payment{CurrencyAmount: 1234.567}: true,
		&Payment{CurrencyAmount: 0}:        false,
		&Payment{CurrencyAmount: -0.1}:     false,
		&Payment{CurrencyAmount: -100}:     false,
	}

	for p, shouldValid := range testPayments {
		isValid := p.HasValidCurrencyAmount()
		suite.Equal(shouldValid, isValid)
	}
}

func (suite *APITestSuite) TestGenerateUniqueIntegratedAddress() {
	w, _ := processor.ActiveWallets.GetWalletFromStoreID(suite.mockStore.ID)

	count := 5
	generatedAddresses := make([]string, count)

	for i := 0; i < count; i++ {
		iaddr, _, err := GenerateUniqueIntegratedAddress(w)
		suite.Nil(err)
		suite.NotContains(generatedAddresses, iaddr)

		generatedAddresses = append(generatedAddresses, iaddr)
	}
}

func (suite *APITestSuite) TestCalculateTTL() {
	oldMaxTTL := config.PaymentMaxTTL

	config.PaymentMaxTTL = 60

	test := []struct {
		Payment          *Payment
		MinsFromCreation int
		ExpectedTTL      int
	}{
		{Payment: &Payment{Status: processor.PaymentStatusPending}, MinsFromCreation: 0, ExpectedTTL: 60 - 0},
		{Payment: &Payment{Status: processor.PaymentStatusPending}, MinsFromCreation: 1, ExpectedTTL: 60 - 1},
		{Payment: &Payment{Status: processor.PaymentStatusPending}, MinsFromCreation: 20, ExpectedTTL: 60 - 20},
		{Payment: &Payment{Status: processor.PaymentStatusPending}, MinsFromCreation: 60, ExpectedTTL: 0},
		{Payment: &Payment{Status: processor.PaymentStatusPending}, MinsFromCreation: 100, ExpectedTTL: 0},
		{Payment: &Payment{Status: processor.PaymentStatusPaid}, MinsFromCreation: 1000, ExpectedTTL: 0},
	}

	for _, t := range test {
		t.Payment.CalculateTTL(t.MinsFromCreation)
		suite.Equal(t.ExpectedTTL, t.Payment.TTL)
	}

	config.PaymentMaxTTL = oldMaxTTL
}

func (suite *APITestSuite) TestPayments() {
	testPayments := []struct {
		Currency       string
		CurrencyAmount float64
		StoreID        int

		Payment         *Payment
		ExpectedErrCode int
		ExpectedErr     error
	}{
		{Currency: "DERO", CurrencyAmount: 50, StoreID: suite.mockStore.ID, ExpectedErrCode: 0, ExpectedErr: nil},
		{Currency: "EUR", CurrencyAmount: 10, StoreID: suite.mockStore.ID, ExpectedErrCode: 0, ExpectedErr: nil},
		{Currency: "ABC", CurrencyAmount: 10, StoreID: suite.mockStore.ID, ExpectedErrCode: http.StatusUnprocessableEntity, ExpectedErr: ErrInvalidCurrency},
		{Currency: "USD", CurrencyAmount: 0, StoreID: suite.mockStore.ID, ExpectedErrCode: http.StatusUnprocessableEntity, ExpectedErr: ErrInvalidAmount},
	}

	var (
		errCode int
		err     error

		validPaymentIDs []string
	)
	for _, p := range testPayments {
		// Test CreateNewPayment
		p.Payment, _, errCode, err = CreateNewPayment(p.Currency, p.CurrencyAmount, p.StoreID)
		suite.Equal(p.ExpectedErrCode, errCode)
		suite.Equal(p.ExpectedErr, err)

		if err == nil {
			validPaymentIDs = append(validPaymentIDs, p.Payment.PaymentID)

			// Test Insert
			err = p.Payment.Insert()
			suite.Nil(err)
			suite.NotZero(p.Payment.CreationTime)

			// Test FetchPaymentFromID
			_, errCode, err = FetchPaymentFromID(p.Payment.PaymentID, p.Payment.StoreID)
			suite.Zero(errCode)
			suite.Nil(err)
		}
	}

	// Test FetchPaymentFromID (payment not found)
	invalidPaymentID, _ := stringutil.RandomHexString(32)
	_, errCode, err = FetchPaymentFromID(invalidPaymentID, suite.mockStore.ID)
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(ErrPaymentNotFound, err)

	// Test FetchPaymentsFromIDs
	_, errCode, err = FetchPaymentsFromIDs(validPaymentIDs, suite.mockStore.ID)
	suite.Zero(errCode)
	suite.Nil(err)

	// Test FetchPaymentsFromIDs (payments not found)
	_, errCode, err = FetchPaymentsFromIDs([]string{invalidPaymentID}, suite.mockStore.ID)
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(ErrPaymentsNotFound, err)
}

func (suite *APITestSuite) TestFetchFilteredPayments() {
	storeID := suite.mockStore.ID
	mockPayments := []*Payment{}
	addMockPayment := func(status string, currency string, amount float64) {
		p, _, _, _ := CreateNewPayment(currency, amount, storeID)
		p.Status = status
		p.Insert()
		mockPayments = append(mockPayments, p)
	}

	// Test fetching payments before adding any
	payments, numPayments, numPages, errCode, err := FetchFilteredPayments(storeID, 0, 1, "creation_time", "desc", "", "")
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(ErrNoPaymentsFound, err)
	suite.Equal(0, numPayments)
	suite.Equal(0, numPages)
	suite.Nil(payments)

	// 9 mock payments
	addMockPayment("pending", "DERO", 10)
	addMockPayment("paid", "DERO", 20)
	addMockPayment("pending", "DERO", 30)
	addMockPayment("paid", "DERO", 40)
	addMockPayment("paid", "USD", 50)
	addMockPayment("pending", "USD", 60)
	addMockPayment("expired", "USD", 70)
	addMockPayment("pending", "EUR", 80)
	addMockPayment("error", "EUR", 90)

	// Test fetching payments one by one
	payments, numPayments, numPages, errCode, err = FetchFilteredPayments(storeID, 1, 1, "creation_time", "desc", "", "")
	suite.Zero(errCode)
	suite.Nil(err)
	suite.Equal(9, numPayments)
	suite.Equal(9, numPages)                             // Because limit = 1
	suite.Equal(float64(90), payments[0].CurrencyAmount) // Last added payment

	// Test fetching all payments
	payments, numPayments, numPages, errCode, err = FetchFilteredPayments(storeID, 0, 1, "creation_time", "desc", "", "")
	suite.Zero(errCode)
	suite.Nil(err)
	suite.Equal(9, numPayments)
	suite.Equal(1, numPages)                             // Because no limit
	suite.Equal(float64(90), payments[0].CurrencyAmount) // Last added payment
	suite.Equal(float64(10), payments[8].CurrencyAmount) // First added payment

	// Test fetching the 3rd page of the 9 payments divided in groups of 3
	payments, numPayments, numPages, errCode, err = FetchFilteredPayments(storeID, 3, 3, "creation_time", "desc", "", "")
	suite.Zero(errCode)
	suite.Nil(err)
	suite.Equal(9, numPayments)
	suite.Equal(3, numPages)
	suite.Equal(float64(30), payments[0].CurrencyAmount)
	suite.Equal(float64(20), payments[1].CurrencyAmount)
	suite.Equal(float64(10), payments[2].CurrencyAmount)

	// Test fetching the 5th page (out of range by 2) of the 9 payments divided in groups of 3
	payments, numPayments, numPages, errCode, err = FetchFilteredPayments(storeID, 3, 5, "creation_time", "desc", "", "")
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(ErrNoPaymentsFoundPage, err)
	suite.Equal(9, numPayments)
	suite.Equal(3, numPages)
	suite.Nil(payments)

	// Test fetching the *only* the *first* *paid* payment in *USD*
	payments, numPayments, numPages, errCode, err = FetchFilteredPayments(storeID, 1, 1, "creation_time", "asc", "paid", "USD")
	suite.Zero(errCode)
	suite.Nil(err)
	suite.Equal(1, numPayments)
	suite.Equal(1, numPages)
	suite.Equal(float64(50), payments[0].CurrencyAmount)
}
