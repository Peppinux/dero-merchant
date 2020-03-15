package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

// ServerPort is the port the web server will listen to
var ServerPort int

// PostgresSQL database config
var (
	DBName     string
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     int
)

// RedisAddress is the host:port of the Redis server the application will connect to
var RedisAddress string

// Dero Network, wallet and payments config
var (
	// DeroNetwork is either mainnet or testnet
	DeroNetwork string
	// DeroDaemonAddress is the host:port of the (possibly remote) node active wallets will connect to
	DeroDaemonAddress string
	// WalletsPath is the relative path of the directory where active wallets files will be stored
	WalletsPath string
	// PaymentMaxTTL is the MAX number of MINUTES allowed to receive payment before it expires
	PaymentMaxTTL int
	// PaymentMinConfirmations is the MINIMUM number of confirmations a payment needs to have before it is considered valid
	PaymentMinConfirmations int
)

// Config for testing
var (
	TestDBName            string
	TestDBUser            string
	TestDBPassword        string
	TestDBHost            string
	TestDBPort            int
	TestRedisAddress      string
	TestDeroNetwork       string
	TestDeroDaemonAddress string
	TestWalletsPath       string
)

// LoadFromENV loads the config from .env file
func LoadFromENV(filenames ...string) error {
	err := godotenv.Load(filenames...)
	if err != nil {
		return errors.Wrap(err, "cannot load godotenv .env file")
	}

	ServerPort, err = strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		return errors.Wrap(err, "cannot convert string to integer")
	}

	DBName = os.Getenv("DB_NAME")
	DBUser = os.Getenv("DB_USER")
	DBPassword = os.Getenv("DB_PASSWORD")
	DBHost = os.Getenv("DB_HOST")
	DBPort, err = strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return errors.Wrap(err, "cannot convert string to integer")
	}

	RedisAddress = os.Getenv("REDIS_ADDRESS")

	DeroNetwork = strings.ToLower(os.Getenv("DERO_NETWORK"))
	DeroDaemonAddress = os.Getenv("DERO_DAEMON_ADDRESS")
	WalletsPath = os.Getenv("WALLETS_PATH")
	PaymentMaxTTL, err = strconv.Atoi(os.Getenv("PAYMENT_MAX_TTL"))
	if err != nil {
		return errors.Wrap(err, "cannot convert string to integer")
	}
	PaymentMinConfirmations, err = strconv.Atoi(os.Getenv("PAYMENT_MIN_CONFIRMATIONS"))
	if err != nil {
		return errors.Wrap(err, "cannot convert string to integer")
	}

	TestDBName = os.Getenv("TEST_DB_NAME")
	TestDBUser = os.Getenv("TEST_DB_USER")
	TestDBPassword = os.Getenv("TEST_DB_PASSWORD")
	TestDBHost = os.Getenv("TEST_DB_HOST")
	TestDBPort, err = strconv.Atoi(os.Getenv("TEST_DB_PORT"))
	if err != nil {
		return errors.Wrap(err, "cannot convert string to integer")
	}
	TestRedisAddress = os.Getenv("TEST_REDIS_ADDRESS")
	TestDeroNetwork = strings.ToLower(os.Getenv("TEST_DERO_NETWORK"))
	TestDeroDaemonAddress = os.Getenv("TEST_DERO_DAEMON_ADDRESS")
	TestWalletsPath = os.Getenv("TEST_WALLETS_PATH")

	return nil
}
