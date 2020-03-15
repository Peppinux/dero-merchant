package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/peppinux/dero-merchant/api"
	"github.com/peppinux/dero-merchant/auth"
	"github.com/peppinux/dero-merchant/coingecko"
	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/processor"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/webapp"
	"github.com/peppinux/dero-merchant/webapp/store"
	"github.com/peppinux/dero-merchant/webapp/user"
)

func main() {
	// Logger setup (logs to both shell and file)
	os.Mkdir("./logs/", 0775)
	logFilePath := fmt.Sprintf("./logs/%v.log", time.Now())
	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Fatalln("Error creating log file:", err)
	}
	gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)
	log.SetOutput(gin.DefaultWriter)
	log.Println("Logger set up.")

	// Load configuration
	err = config.LoadFromENV("./.env")
	if err != nil {
		log.Fatalln("Config: error loading .env file:", err)
	}
	log.Println("Config: loaded from .env file.")

	// PostgreSQL init
	postgres.DB, err = postgres.Connect(config.DBName, config.DBUser, config.DBPassword, config.DBHost, config.DBPort, "disable") // TODO: Enable SSLMode?
	if err != nil {
		log.Fatalln("Error connecting to PostgresSQL database:", err)
	}
	defer postgres.DB.Close()
	err = postgres.DB.Ping()
	if err != nil {
		log.Fatalln("PostgreSQL Server: OFFLINE.")
	} else {
		log.Println("PostgreSQL Server: ONLINE.")
	}
	postgres.CreateTablesIfNotExist()

	// Redis init
	redis.Pool = redis.NewPool(config.RedisAddress)
	defer redis.Pool.Close()
	err = redis.Ping()
	if err != nil {
		log.Fatalln("Redis Server: OFFLINE.")
	} else {
		log.Println("Redis Server: ONLINE.")
	}
	redis.FlushAll()

	// CoinGecko API V3 server status check
	statusCode := coingecko.Ping()
	if statusCode == http.StatusOK {
		log.Println("CoinGecko API V3: ONLINE.")
	} else {
		log.Fatalln("CoinGecko API V3: OFFLINE.")
	}

	// Payment processor init
	processor.ActiveWallets = processor.NewStoresWallets()
	err = processor.SetupDaemonConnection()
	if err != nil {
		log.Fatalf("Error setting up connection to daemon %s: %v\n", config.DeroDaemonAddress, err)
	}
	log.Printf("DERO Network: Connected to %s daemon %s\n", config.DeroNetwork, config.DeroDaemonAddress)
	err = processor.CreateWalletsDirectory()
	if err != nil {
		log.Fatalln("Error creating wallets directory:", err)
	}
	log.Println("Wallets directory created.")

	// In case of application restart after a crash, update the status of pending payments to "error"
	// since the application could have not been able to check for them.
	err = processor.CleanAllPendingPayments()
	if err != nil {
		log.Println("Error cleaning all pending payments:", err)
	}

	// Router init
	r := gin.Default()

	r.Use(gzip.Gzip(gzip.DefaultCompression))

	r.Static("/static", "./webassets/static")
	r.StaticFile("/license", "./LICENSE")
	r.StaticFile("/docs", "./documentation/docs.html")

	r.LoadHTMLGlob("./webassets/templates/**/*")

	r.NoRoute(webapp.Error404Handler)

	// Web App routes
	web := r.Group("", auth.SessionAuth())
	{
		web.GET("/", webapp.IndexHandler)

		userGroup := web.Group("/user")
		{
			notAuthOrRedirect := userGroup.Group("", auth.SessionNotAuthOrRedirect())
			{
				notAuthOrRedirect.GET("/signup", user.SignUpGetHandler)
				notAuthOrRedirect.POST("/signup", user.SignUpPostHandler)

				notAuthOrRedirect.GET("/signin", user.SignInGetHandler)
				notAuthOrRedirect.POST("/signin", user.SignInPostHandler)

				notAuthOrRedirect.GET("/forgot_password", user.ForgotPasswordGetHandler)
				notAuthOrRedirect.POST("/forgot_password", user.ForgotPasswordPostHandler)
				notAuthOrRedirect.GET("/recover", user.RecoverGetHandler)
				notAuthOrRedirect.POST("/recover", user.RecoverPostHandler)
			}

			authOrRedirect := userGroup.Group("", auth.SessionAuthOrRedirect())
			{
				authOrRedirect.POST("/signout", user.SignOutHandler)
				authOrRedirect.POST("/signout_all", user.SignOutAllHandler)
			}

			userGroup.GET("/verify", user.VerifyHandler)
			userGroup.GET("/new_verification_token", user.NewVerificationTokenGetHandler)
			userGroup.POST("/new_verification_token", user.NewVerificationTokenPostHandler)

			authOrForbidden := userGroup.Group("", auth.SessionAuthOrForbidden())
			{
				authOrForbidden.PUT("", user.PutHandler)
			}
		}

		dashboard := web.Group("/dashboard", auth.SessionAuthOrRedirect())
		{
			dashboard.GET("", webapp.DashboardHandler)
			dashboard.GET("/account", webapp.MyAccountHandler)

			stores := dashboard.Group("/stores")
			{
				stores.GET("", webapp.MyStoresHandler)
				stores.GET("/view/:id", webapp.ViewStoreHandler)
				stores.GET("/add", webapp.AddStoreGetHandler)
				stores.POST("/add", webapp.AddStorePostHandler)
				stores.GET("/view/:id/payments", webapp.ViewStorePaymentsHandler)
			}
		}

		storeGroup := web.Group("/store", auth.SessionAuthOrForbidden())
		{
			storeGroup.GET("/:id/payments", store.PaymentsGetHandler)

			requirePassword := storeGroup.Group("", auth.RequireUserPassword())
			{
				requirePassword.PUT("/:id", store.PutHandler)
				requirePassword.DELETE("/:id", store.DeleteHandler)
			}
		}
	}

	// Pay helper endpoint for customers
	r.GET("/pay/:payment_id", webapp.PayHandler)
	// Web Socket handler used by /pay/:payment_id to update payment's status on page
	r.GET("/ws/payment/:payment_id/status", webapp.WSPaymentStatusHandler)

	// API routes
	apiGroup := r.Group("/api")
	{
		v1 := apiGroup.Group("/v1", auth.APIKeyAuth())
		{
			v1.GET("/ping", api.PingGetHandler)

			payment := v1.Group("/payment")
			{
				requireSecretKey := payment.Group("", auth.SecretKeyAuth())
				{
					requireSecretKey.POST("", api.PaymentPostHandler)
				}

				payment.GET("/:payment_id", api.PaymentGetHandler)
			}

			v1.POST("/payments", api.PaymentsPostHandler)
			v1.GET("/payments", api.PaymentsGetHandler)
		}
	}

	// Secure web server configuration suggested on CloudFlare Blog https://blog.cloudflare.com/exposing-go-on-the-internet/
	/*tlsConfig := &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
	}*/

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.ServerPort),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		//TLSConfig:    tlsConfig,
		Handler: r,
	}

	go func() {
		// TODO: Add TLS, edit ServerPort in 443. Redirect http requests to https using unrolled/secure? Also Edit ws in wss pay.js
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln("Error running server:", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Cleaning all pending payments...")
	err = processor.CleanAllPendingPayments()
	if err != nil {
		log.Println("Error cleaning all pending payments:", err)
	}

	log.Println("Gracefully shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalln("Error shutting down server:", err)
	}

	log.Println("Server shut down.")
}
