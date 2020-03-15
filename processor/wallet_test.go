// This test is desabled because it requires manual intervention.
// Uncomment func (suite *WalletTestSuite) TestPaymentProcessor() and set the test -timeout flag to PAYMENT_MAX_TTL mins to execute it.

package processor

import (
	"os"
	"testing"
	"time"

	"github.com/alexedwards/argon2id"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
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

type PaymentMock struct {
	PaymentID         string
	Status            string
	Currency          string
	CurrencyAmount    float64
	ExchangeRate      float64
	DeroAmount        string
	AtomicDeroAmount  uint64
	IntegratedAddress string
	CreationTime      time.Time
	StoreID           int
}

type WalletTestSuite struct {
	suite.Suite

	mockUsers    []*UserMock
	mockStores   []*StoreMock
	mockPayments []*PaymentMock
}

func (suite *WalletTestSuite) SetupSuite() {
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

	postgres.DB, err = postgres.Connect(config.TestDBName, config.TestDBUser, config.TestDBPassword, config.TestDBHost, config.TestDBPort, "disable") // TODO: Gestire SSLMode
	if err != nil {
		panic(err)
	}

	postgres.DropTables()
	postgres.CreateTablesIfNotExist()

	config.DeroNetwork = config.TestDeroNetwork
	config.DeroDaemonAddress = config.TestDeroDaemonAddress
	ActiveWallets = NewStoresWallets()
	err = SetupDaemonConnection()
	if err != nil {
		panic(err)
	}

	config.WalletsPath = config.TestWalletsPath
	err = CreateWalletsDirectory()
	if err != nil {
		panic(err)
	}

	suite.mockUsers = append(suite.mockUsers, &UserMock{
		Username: "Test User 1",
		Email:    "first@foobar.baz",
		Password: "foobarbaz",
		Verified: true,
	})

	suite.mockUsers = append(suite.mockUsers, &UserMock{
		Username: "Test User 2",
		Email:    "second@foobar.baz",
		Password: "barbazfoo",
		Verified: true,
	})

	for _, u := range suite.mockUsers {
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
	}

	suite.mockStores = append(suite.mockStores, &StoreMock{
		Title:   "First store owned by User 1",
		ViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c",
		Webhook: "/store1by1_webhook",
		OwnerID: suite.mockUsers[0].ID,
	})

	suite.mockStores = append(suite.mockStores, &StoreMock{
		Title:   "Second store owned by User 1",
		ViewKey: "b840fc183d93caf070b6ebf82499fa106485737978fc3862149d203402196631837f5128955c1f0fb1500e2aa429c2ba72268fb062a54947ec6f35045170c20c",
		Webhook: "/store2by1_webhook",
		OwnerID: suite.mockUsers[0].ID,
	})

	suite.mockStores = append(suite.mockStores, &StoreMock{
		Title:   "Store owned by User 2",
		ViewKey: "5c0b566aa397d94befbe52655c3417e1c4597249b9625f30d08e91f6a0c0bf0b47bf735d614b3e62ad4db84831c027d0b82cd4eea11026c6f3d8c892f5865501",
		Webhook: "/storeby2_webhook",
		OwnerID: suite.mockUsers[1].ID,
	})

	for _, s := range suite.mockStores {
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
	}

	suite.mockPayments = append(suite.mockPayments, &PaymentMock{
		Status:           "pending",
		Currency:         "DERO",
		CurrencyAmount:   1,
		ExchangeRate:     1,
		DeroAmount:       "1.000000000000",
		AtomicDeroAmount: 1000000000000,
		StoreID:          suite.mockStores[0].ID,
	})

	suite.mockPayments = append(suite.mockPayments, &PaymentMock{
		Status:           "pending",
		Currency:         "DERO",
		CurrencyAmount:   1,
		ExchangeRate:     1,
		DeroAmount:       "1.000000000000",
		AtomicDeroAmount: 1000000000000,
		StoreID:          suite.mockStores[1].ID,
	})

	suite.mockPayments = append(suite.mockPayments, &PaymentMock{
		Status:           "pending",
		Currency:         "DERO",
		CurrencyAmount:   1,
		ExchangeRate:     1,
		DeroAmount:       "1.000000000000",
		AtomicDeroAmount: 1000000000000,
		StoreID:          suite.mockStores[2].ID,
	})

	suite.mockPayments = append(suite.mockPayments, &PaymentMock{
		Status:           "pending",
		Currency:         "DERO",
		CurrencyAmount:   1,
		ExchangeRate:     1,
		DeroAmount:       "1.000000000000",
		AtomicDeroAmount: 1000000000000,
		StoreID:          suite.mockStores[2].ID,
	})

	for _, p := range suite.mockPayments {
		w, err := ActiveWallets.GetWalletFromStoreID(p.StoreID)
		if err != nil {
			panic(err)
		}

		p.IntegratedAddress, p.PaymentID = w.GenerateIntegratedAddress()

		err = postgres.DB.QueryRow(`
			INSERT INTO payments (payment_id, status, currency, currency_amount, exchange_rate, dero_amount, atomic_dero_amount, integrated_address, store_id) 
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9) 
			RETURNING creation_time`, p.PaymentID, p.Status, p.Currency, p.CurrencyAmount, p.ExchangeRate, p.DeroAmount, p.AtomicDeroAmount, p.IntegratedAddress, p.StoreID).
			Scan(&p.CreationTime)
		if err != nil {
			panic(err)
		}
	}
}

func (suite *WalletTestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()

	postgres.DropTables()
	postgres.DB.Close()

	os.RemoveAll(config.TestWalletsPath)
}

func TestWalletTestSuite(t *testing.T) {
	suite.Run(t, new(WalletTestSuite))
}

/*func (suite *WalletTestSuite) TestPaymentProcessor() {
	fmt.Println("This part of testing requires manual intervention.")
	fmt.Println("Execute the following actions to continue:")

	for _, p := range suite.mockPayments {
		fmt.Printf("- Send %s DERO to %s\n", p.DeroAmount, p.IntegratedAddress)
		fmt.Println(p.StoreID, p.PaymentID) // TODO: eliminare
	}

	for _, p := range suite.mockPayments {
		w, err := ActiveWallets.GetWalletFromStoreID(p.StoreID)
		suite.Nil(err)

		err = w.AddPendingPayment(p.PaymentID, p.AtomicDeroAmount)
		suite.Nil(err)
	}

	var minutesPassed int
	for range time.NewTicker(time.Minute).C {
		log.Println("YO 1 MINUTE PASSED YO TODO ELIMINARE")

		if minutesPassed >= config.PaymentMaxTTL {
			suite.Fail("Payments not received in time.")
		}

		ActiveWallets.Mutex.RLock()
		defer ActiveWallets.Mutex.RUnlock()

		someWalletStillWaiting := false
		for _, w := range ActiveWallets.Map {
			if w.PendingPayments.Count() > 0 {
				someWalletStillWaiting = true
			}
		}

		if !someWalletStillWaiting {
			break
		}

		minutesPassed++
	}
}*/
