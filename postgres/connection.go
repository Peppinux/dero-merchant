package postgres

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
)

// DB is the gloabl PostgreSQL connection opened in main
var DB *sql.DB

// Connect opens a new connection to a PostgreSQL database
func Connect(name string, user string, password string, host string, port int, sslmode string) (db *sql.DB, err error) {
	connStr := fmt.Sprintf("dbname=%s user=%s password=%s host=%s port=%d sslmode=%s", name, user, password, host, port, sslmode)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		err = errors.Wrap(err, "cannot open database connection")
	}
	return
}

// CreateTablesIfNotExist creates the tables necessary to the application if they do not already exist in DB
func CreateTablesIfNotExist() {
	const (
		usersTable = `
				CREATE TABLE IF NOT EXISTS users
				(
					id integer NOT NULL GENERATED BY DEFAULT AS IDENTITY (INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1),
					username character varying(16) NOT NULL,
					email character varying(64) NOT NULL,
					password character varying(128) NOT NULL,
					signup_date timestamp without time zone NOT NULL DEFAULT now(),
					email_verified boolean NOT NULL DEFAULT false,
					verification_token character(64) NOT NULL,
					verification_token_expiration_date timestamp without time zone NOT NULL DEFAULT (now() + '01:00:00'::interval),
					recover_token character(64) DEFAULT NULL::bpchar,
					recover_token_expiration_date timestamp without time zone,
					CONSTRAINT users_pkey PRIMARY KEY (id),
					CONSTRAINT users_username_key UNIQUE (username),
					CONSTRAINT users_email_key UNIQUE (email),
					CONSTRAINT users_verification_token_key UNIQUE (verification_token),
					CONSTRAINT users_recover_token_key UNIQUE (recover_token)
				);
				`
		storesTable = `
				CREATE TABLE IF NOT EXISTS stores
				(
					id integer NOT NULL GENERATED BY DEFAULT AS IDENTITY (INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1),
					title character varying(64) NOT NULL,
					wallet_view_key character(128) NOT NULL,
					webhook character varying NOT NULL DEFAULT '',
					webhook_secret_key character(64) NOT NULL,
					api_key character(64) NOT NULL,
					secret_key character(64) NOT NULL,
					removed boolean NOT NULL DEFAULT false,
					owner_id integer NOT NULL,
					CONSTRAINT stores_pkey PRIMARY KEY (id),
					CONSTRAINT stores_webhook_secret_key_key UNIQUE (webhook_secret_key),
					CONSTRAINT stores_api_key_key UNIQUE (api_key),
					CONSTRAINT stores_secret_key_key UNIQUE (secret_key),
					CONSTRAINT stores_owner_id_fkey FOREIGN KEY (owner_id)
						REFERENCES public.users (id) MATCH SIMPLE
						ON UPDATE NO ACTION
						ON DELETE NO ACTION
						NOT VALID
				);
				`
		paymentsTable = `
				CREATE TABLE IF NOT EXISTS payments
				(
					payment_id character(64) NOT NULL,
					status character varying NOT NULL,
					currency character varying NOT NULL,
					currency_amount double precision NOT NULL,
					exchange_rate double precision NOT NULL,
					dero_amount character varying NOT NULL,
					atomic_dero_amount bigint NOT NULL,
					integrated_address character(142) NOT NULL,
					creation_time timestamp without time zone NOT NULL DEFAULT now(),
					store_id integer NOT NULL,
					CONSTRAINT payments_pkey PRIMARY KEY (payment_id),
					CONSTRAINT payments_payment_id_key UNIQUE (payment_id),
					CONSTRAINT payments_integrated_address_key UNIQUE (integrated_address),
					CONSTRAINT payments_store_id_fkey FOREIGN KEY (store_id)
						REFERENCES public.stores (id) MATCH SIMPLE
						ON UPDATE NO ACTION
						ON DELETE NO ACTION
						NOT VALID
				);
				`
	)

	DB.Exec(usersTable)
	DB.Exec(storesTable)
	DB.Exec(paymentsTable)
}

// DropTables DROPS ALL tables in DB
func DropTables() {
	DB.Exec("DROP TABLE payments;")
	DB.Exec("DROP TABLE stores;")
	DB.Exec("DROP TABLE users;")
}
