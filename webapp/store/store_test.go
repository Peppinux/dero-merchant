package store

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/alexedwards/argon2id"
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

type StoreTestSuite struct {
	suite.Suite

	mockUser *UserMock
}

func (suite *StoreTestSuite) SetupSuite() {
	err := config.LoadFromENV("../../.env")
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

	suite.mockUser = &UserMock{
		Username: "Test owner",
		Email:    "foobar@baz.com",
		Password: "foobarbaz",
		Verified: true,
	}

	u := suite.mockUser

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

func (suite *StoreTestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()

	postgres.DropTables()
	postgres.DB.Close()
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) TestHasValidTitle() {
	testStores := map[*Store]bool{
		&Store{Title: "Amazing Store"}:                          true,
		&Store{Title: "Bay of Products"}:                        true,
		&Store{Title: "Productslist"}:                           true,
		&Store{Title: ""}:                                       false,
		&Store{Title: "The title of this store is way to long"}: false,
	}

	for s, shouldValid := range testStores {
		isValid := s.HasValidTitle()
		suite.Equal(shouldValid, isValid)
	}
}

func (suite *StoreTestSuite) TestHasValidViewKey() {
	testStores := map[*Store]bool{
		&Store{WalletViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c"}:  true,
		&Store{WalletViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0"}:   false,
		&Store{WalletViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0ca"}: false,
		&Store{WalletViewKey: ""}: false,
	}

	for s, shouldValid := range testStores {
		isValid := s.HasValidViewKey()
		suite.Equal(shouldValid, isValid)
	}
}

func (suite *StoreTestSuite) TestHasUniqueTitle() {
	ownerID := suite.mockUser.ID

	testTitles := []string{
		"Test store title",
		"Another test title",
		"Original store title",
	}

	duplicateTitles := []string{
		"Test store title",
		"Original store title",
	}

	for _, t := range testTitles {
		s, _ := CreateNewStore(t, "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
		isUnique, err := s.HasUniqueTitle()
		suite.Nil(err)
		suite.True(isUnique)

		s.Insert()
	}

	for _, t := range duplicateTitles {
		s, _ := CreateNewStore(t, "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
		isUnique, err := s.HasUniqueTitle()
		suite.Nil(err)
		suite.False(isUnique)
	}
}

func (suite *StoreTestSuite) TestGenerateUniqueStoreKeys() {
	count := 5
	generatedAPIKeys := make([]string, count)

	for i := 0; i < count; i++ {
		apiKey, secretKey, err := GenerateUniqueStoreKeys()
		suite.Nil(err)
		suite.Len(apiKey, 64)
		suite.Len(secretKey, 64)
		suite.NotContains(generatedAPIKeys, apiKey)

		generatedAPIKeys = append(generatedAPIKeys, apiKey)
	}
}

func (suite *StoreTestSuite) TestGenerateUniqueWebhookSecretKey() {
	count := 5
	generatedKeys := make([]string, count)

	for i := 0; i < count; i++ {
		secretKey, err := GenerateUniqueWebhookSecretKey()
		suite.Nil(err)
		suite.Len(secretKey, 64)
		suite.NotContains(generatedKeys, secretKey)

		generatedKeys = append(generatedKeys, secretKey)
	}
}

func (suite *StoreTestSuite) TestStores() {
	ownerID := suite.mockUser.ID

	testStores := []struct {
		Title   string
		ViewKey string
		Webhook string

		Store        *Store
		ExpectedErrs []error
	}{
		{Title: "A great store", ViewKey: "", ExpectedErrs: []error{ErrInvalidViewKey}},
		{Title: "", ViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", ExpectedErrs: []error{ErrInvalidTitle}},
		{Title: "", ViewKey: "", ExpectedErrs: []error{ErrInvalidTitle, ErrInvalidViewKey}},
		{Title: "A great store", ViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", ExpectedErrs: nil},
		{Title: "A great store", ViewKey: "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", ExpectedErrs: []error{ErrTitleNotUnique}},
	}

	var (
		errs []error

		lastStoreID int
	)
	for _, s := range testStores {
		// Test CreateNewStore
		s.Store, errs = CreateNewStore(s.Title, s.ViewKey, s.Webhook, ownerID)
		suite.Equal(s.ExpectedErrs, errs)

		if errs == nil {
			// Test Insert
			err := s.Store.Insert()
			suite.Nil(err)

			lastStoreID = s.Store.ID

			// Test FetchStoreFromID
			_, errCode, err := FetchStoreFromID(s.Store.ID, ownerID)
			suite.Zero(errCode)
			suite.Nil(err)
		}
	}

	// Test FetchStoreFromID (store not found)
	_, errCode, err := FetchStoreFromID(lastStoreID+123, ownerID)
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(sql.ErrNoRows, err)

	// Test FetchStoreFromID (invalid owner)
	_, errCode, err = FetchStoreFromID(lastStoreID, ownerID+123)
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(sql.ErrNoRows, err)
}

func (suite *StoreTestSuite) TestUpdateViewKey() {
	ownerID := suite.mockUser.ID

	s, _ := CreateNewStore("Store to be updated 1", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
	s.Insert()

	testViewKeys := []struct {
		NewViewKey string

		ExpectedErrCode int
		ExpectedErr     error
	}{
		{NewViewKey: "", ExpectedErrCode: http.StatusUnprocessableEntity, ExpectedErr: ErrInvalidViewKeyVerbose},
		{NewViewKey: "c53d44b598141c5527ab6a39e", ExpectedErrCode: http.StatusUnprocessableEntity, ExpectedErr: ErrInvalidViewKeyVerbose},
		{NewViewKey: "d64d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b760f1d", ExpectedErrCode: 0, ExpectedErr: nil},
	}

	for _, vk := range testViewKeys {
		errCode, err := s.UpdateViewKey(vk.NewViewKey)
		suite.Equal(vk.ExpectedErrCode, errCode)
		suite.Equal(vk.ExpectedErr, err)
	}

	s, _, _ = FetchStoreFromID(s.ID, ownerID)
	suite.Equal(testViewKeys[2].NewViewKey, s.WalletViewKey)
}

func (suite *StoreTestSuite) TestUpdateWebhook() {
	ownerID := suite.mockUser.ID

	url := ""
	s, _ := CreateNewStore("Store to be updated 2", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", url, ownerID)
	s.Insert()

	s, _, _ = FetchStoreFromID(s.ID, ownerID)
	suite.Equal(url, s.Webhook)

	newURL := "test.com/webhook"
	errCode, err := s.UpdateWebhook(newURL)
	suite.Zero(errCode)
	suite.Nil(err)

	s, _, _ = FetchStoreFromID(s.ID, ownerID)
	suite.Equal(newURL, s.Webhook)
}

func (suite *StoreTestSuite) TestUpdateWebhookSecretKey() {
	ownerID := suite.mockUser.ID

	s, _ := CreateNewStore("Store to be updated 3", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
	s.Insert()

	oldSecretKey := s.WebhookSecretKey

	s, _, _ = FetchStoreFromID(s.ID, ownerID)
	suite.Equal(oldSecretKey, s.WebhookSecretKey)

	errCode, err := s.UpdateWebhookSecretKey()
	suite.Zero(errCode)
	suite.Nil(err)

	suite.NotEqual(oldSecretKey, s.WebhookSecretKey)
}

func (suite *StoreTestSuite) TestUpdateKeys() {
	ownerID := suite.mockUser.ID

	s, _ := CreateNewStore("Store to be updated 4", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
	s.Insert()

	oldAPIKey, oldSecretKey := s.APIKey, s.SecretKey
	s, _, _ = FetchStoreFromID(s.ID, ownerID)
	suite.Equal(oldAPIKey, s.APIKey)
	suite.Equal(oldSecretKey, s.SecretKey)

	errCode, err := s.UpdateKeys()
	suite.Zero(errCode)
	suite.Nil(err)

	suite.NotEqual(oldAPIKey, s.APIKey)
	suite.NotEqual(oldSecretKey, s.SecretKey)
}

func (suite *StoreTestSuite) TestUpdateByInvalidUser() {
	ownerID := suite.mockUser.ID

	s, _ := CreateNewStore("Store to be updated 5", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
	s.Insert()

	s.OwnerID += 123

	errCode, err := s.UpdateViewKey("d64d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b760f1d")
	suite.Equal(errCode, http.StatusForbidden)
	suite.Equal(err, ErrForbidden)

	errCode, err = s.UpdateWebhook("test.com/webhook")
	suite.Equal(errCode, http.StatusForbidden)
	suite.Equal(err, ErrForbidden)

	errCode, err = s.UpdateWebhookSecretKey()
	suite.Equal(errCode, http.StatusForbidden)
	suite.Equal(err, ErrForbidden)

	errCode, err = s.UpdateKeys()
	suite.Equal(errCode, http.StatusForbidden)
	suite.Equal(ErrForbidden, err)
}

func (suite *StoreTestSuite) TestRemove() {
	ownerID := suite.mockUser.ID

	s, _ := CreateNewStore("Store to be removed", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
	s.Insert()

	_, errCode, err := FetchStoreFromID(s.ID, ownerID)
	suite.Zero(errCode)
	suite.Nil(err)

	// Test remove existing store
	_, err = s.Remove()
	suite.Nil(err)

	_, errCode, err = FetchStoreFromID(s.ID, ownerID)
	suite.Equal(http.StatusNotFound, errCode)
	suite.Equal(sql.ErrNoRows, err)

	// Test remove store that has been removed
	_, err = s.Remove()
	suite.NotNil(err)
}

func (suite *StoreTestSuite) TestRemoveByInvalidUser() {
	ownerID := suite.mockUser.ID

	s, _ := CreateNewStore("Store to be removed 2", "c53d44b598141c5527ab6a39e82e107d09620fda2af8c9bdc6cb06db2d4ff368cd73811194dbe53cbbe375fd3d9dc1ad1e334f56726d1289a8c096a13b76fd0c", "", ownerID)
	s.Insert()

	s.OwnerID += 123

	errCode, err := s.Remove()
	suite.Equal(http.StatusForbidden, errCode)
	suite.Equal(ErrForbidden, err)
}
