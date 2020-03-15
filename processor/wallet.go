package processor

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/pkg/errors"

	derowallet "github.com/deroproject/derosuite/walletapi"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/stringutil"
)

// ActiveWallets is the global variable that stores active wallets associated to their StoreIDs
// It is set in main
var ActiveWallets *StoresWallets

// CreateWalletsDirectory creates the directory where active wallets' files will be stored
func CreateWalletsDirectory() error {
	os.RemoveAll(config.WalletsPath)
	err := os.Mkdir(config.WalletsPath, 0775)
	if err != nil {
		return errors.Wrap(err, "cannot make directory")
	}

	return nil
}

// StoreWallet represents the wallet of a store.
// It holds an istance to the actual Dero wallet,
// a list of payments the wallet is supposed to receive
// and the info of the webhook (if set) where payment update events will be sent to
type StoreWallet struct {
	DeroWallet      *derowallet.Wallet
	PendingPayments *PendingPayments
	Webhook         Webhook
}

// NewStoreWallet returns a new StoreWallet struct
func NewStoreWallet(filename string, viewKey string, webhook Webhook) (w *StoreWallet, err error) {
	// Generate random password for wallet file encryption
	password, err := stringutil.RandomHexString(32)
	if err != nil {
		err = errors.Wrap(err, "cannot generate random hex string")
		return
	}

	// Create the actual Dero wallet from View Key
	dw, err := derowallet.Create_Encrypted_Wallet_ViewOnly(filename, password, viewKey)
	if err != nil {
		err = errors.Wrap(err, "cannot create encrypted view onyl Dero wallet")
		return
	}

	// Set Dero wallet Daemon Address
	dw.SetDaemonAddress(config.DeroDaemonAddress)

	w = &StoreWallet{
		DeroWallet:      dw,
		PendingPayments: NewPendingPayments(),
		Webhook:         webhook,
	}

	return
}

// GenerateIntegratedAddress returns random Integrated Address and Payment ID of a wallet
func (w *StoreWallet) GenerateIntegratedAddress() (integratedAddress string, paymentID string) {
	addr := w.DeroWallet.GetRandomIAddress32()
	integratedAddress = addr.String()
	paymentID = hex.EncodeToString(addr.PaymentID)
	return
}

// AddPendingPayment adds a new pending payment the store wallet expects to receive
func (w *StoreWallet) AddPendingPayment(paymentID string, atomicDeroAmount uint64) error {
	err := w.DeroWallet.IsDaemonOnline()
	if err != nil {
		return errors.Wrap(err, "daemon offline")
	}

	p := NewPendingPayment(atomicDeroAmount)
	w.PendingPayments.Set(paymentID, p)

	paymentsCount := w.PendingPayments.Count()

	fmt.Println("DEBUG: Payment ID", paymentID, "added to PendingPayments map.")
	fmt.Println("DEBUG: Map length:", paymentsCount)

	// Make sure store wallet actively checks for new payments every minute
	if paymentsCount == 1 { // If no payments previous than this one are pending, start checking for payments
		err := w.StartCheckingForPayments()
		if err != nil {
			return errors.Wrap(err, "cannot start checking for payments")
		}

		fmt.Println("DEBUG: PendingPayments: 1. Therefore, created wallet and started checking.")
	}
	return nil
}

// StartCheckingForPayments initializes the Dero wallet and starts checking for pending payments
func (w *StoreWallet) StartCheckingForPayments() error {
	err := w.DeroWallet.IsDaemonOnline()
	if err != nil {
		return errors.Wrap(err, "daemon offline")
	}

	/*
		NOTE:

		Wallet sync from specific initial height is possible outside of GOOS=JS
		because we are using a slightly edited Sync_Wallet_With_Daemon function
		that gets rid of GOOS check when setting wallet initial height
	*/
	initialHeight := int64(w.DeroWallet.Get_Daemon_TopoHeight())
	w.DeroWallet.SetInitialHeight(initialHeight)
	w.DeroWallet.SetOnlineMode()

	w.PendingPayments.StartChecker()
	go w.CheckForPayments()
	return nil
}

// StopCheckingForPayments stops the ticker responsible for the checking of pending payments
// and stops the sync of the actual Dero wallet
func (w *StoreWallet) StopCheckingForPayments() {
	w.DeroWallet.SetOfflineMode()
	w.DeroWallet.Clean()
	w.PendingPayments.Checker.Stop()
}

// CheckForPayments checks every minute if wallet received new payments matching the Payment ID of pending payments,
// and updates the payment's status accordingly.
// When all pending payments are paid or expired, it stops the loop
func (w *StoreWallet) CheckForPayments() {
	fmt.Println("DEBUG: Check for payments started")

	for t := range w.PendingPayments.Checker.C {
		fmt.Println("DEBUG: 1 minute passed", t)
		fmt.Println("DEBUG: About to loop PendingPayments")

		err := w.DeroWallet.IsDaemonOnline()
		if err != nil {
			log.Println("Daemon offline. Skipped current loop of payments checking while waiting for it to return online.")
			continue
		}

		fmt.Println("DEBUG: WALLET H/TH", w.DeroWallet.Get_Height(), w.DeroWallet.Get_TopoHeight())
		fmt.Println("DEBUG: DAEMON H/TH", w.DeroWallet.Get_Daemon_Height(), w.DeroWallet.Get_Daemon_TopoHeight())

		for paymentID, payment := range w.PendingPayments.Map {
			fmt.Println("DEBUG: In loop Payment ID", paymentID)

			var (
				receivedAmount uint64
				confirmations  uint64
				notConfirmed   bool
			)

			payid, _ := hex.DecodeString(paymentID)
			entries := w.DeroWallet.Get_Payments_Payment_ID(payid, 0)
			for _, e := range entries {
				receivedAmount += e.Amount

				confirmations = w.DeroWallet.Get_Daemon_Height() - e.Height
				if confirmations < uint64(config.PaymentMinConfirmations) {
					notConfirmed = true
				}
			}

			if notConfirmed { // Payment(s) does not have enough confirmations
				continue
			}

			var newStatus string
			if receivedAmount >= payment.AtomicDeroAmount { // Wallet received payment
				newStatus = PaymentStatusPaid
			} else {
				minsFromCreation := payment.MinutesFromCreation()
				if minsFromCreation > float64(config.PaymentMaxTTL) { // Wallet did not receive payment in time (ORDER_MAX_TTL env variable)
					heightDifference := w.DeroWallet.Get_Daemon_Height() - w.DeroWallet.Get_Height()

					// Make sure wallet is synced with daemon with a tolerance of 20 blocks.
					// Prevents payments that have been received in blocks that are not synced up yet to be market as expired.
					if heightDifference <= 20 {
						newStatus = PaymentStatusExpired
					}
				}
			}

			if newStatus != "" { // Payment status changed
				// Update Payment in DB (Set new status)
				_, err := postgres.DB.Exec(`
					UPDATE payments 
					SET status=$1 
					WHERE payment_id=$2 AND status=$3`, newStatus, paymentID, PaymentStatusPending)
				if err != nil {
					log.Println("Error executing query:", err)
					continue
				}

				// Send payment status update event to store webhook endpoint if set
				if w.Webhook.IsSet() {
					go w.Webhook.SendPaymentUpdateEvent(paymentID, newStatus)
				}

				// Send payment's new status to WebSockets clients (used to update payment status of customer helper page /pay/:payment_id)
				go PaymentWSConnections.SendStatusUpdate(paymentID, newStatus)

				// Delete payment from pending payments since it has either been paid or expired
				w.PendingPayments.Delete(paymentID)
				fmt.Println("DEBUG: Payment removed from map. New status:", newStatus)
			}
		}

		count := w.PendingPayments.Count()
		fmt.Println("DEBUG: PendingPayments map length:", count)
		if count == 0 { // If there are no more pending payments to check the wallet for, clean wallet file and stop the ticker
			w.StopCheckingForPayments()
			fmt.Println("DEBUG: PendingPayments: 0. Therefore, stopped ticker and cleaned wallet.")
			return
		}
	}
}

// CleanAllPendingPayments updates the status of all pending payments to "error".
// This function is supposed to be called only when the application is started (useful after a crash) or gets shut down.
func CleanAllPendingPayments() error {
	for _, w := range ActiveWallets.Map {
		if w.PendingPayments.Count() > 0 {
			w.StopCheckingForPayments()
			for payid := range w.PendingPayments.Map {
				if w.Webhook.IsSet() {
					w.Webhook.SendPaymentUpdateEvent(payid, PaymentStatusError)
				}

				PaymentWSConnections.SendStatusUpdate(payid, PaymentStatusError)
			}
		}
	}
	_, err := postgres.DB.Exec(`
		UPDATE payments
		SET status=$1
		WHERE status=$2`, PaymentStatusError, PaymentStatusPending)
	if err != nil {
		return errors.Wrap(err, "cannot execute query")
	}

	return nil
}

// StoresWallets stores a map of StoreWallet(s) to their store ID and a RWMutex for map synchronization
type StoresWallets struct {
	Map   map[int]*StoreWallet
	Mutex sync.RWMutex
}

// NewStoresWallets returns a new StoresWallets struct
func NewStoresWallets() *StoresWallets {
	return &StoresWallets{
		Map: make(map[int]*StoreWallet),
	}
}

// HasWalletFromStoreID returns whether a StoresWallets map holds the wallet associated to a store ID
func (w *StoresWallets) HasWalletFromStoreID(storeID int) bool {
	w.Mutex.RLock()
	defer w.Mutex.RUnlock()

	return w.Map[storeID] != nil
}

// GetWalletFromStoreID returns the wallet associated to a store ID from a StoresWallet map.
// If wallet does not exist, it gets created
func (w *StoresWallets) GetWalletFromStoreID(storeID int) (*StoreWallet, error) {
	if !w.HasWalletFromStoreID(storeID) {
		err := w.CreateWalletFromStoreID(storeID)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create wallet from store ID")
		}
	}

	w.Mutex.RLock()
	defer w.Mutex.RUnlock()

	return w.Map[storeID], nil
}

// CreateWalletFromStoreID creates a StoreWallet associated to its store ID
func (w *StoresWallets) CreateWalletFromStoreID(storeID int) error {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	// Fetch Wallet View Key and Webhook data from DB
	var (
		viewKey string
		webhook Webhook
	)
	err := postgres.DB.QueryRow(`
		SELECT wallet_view_key, webhook, webhook_secret_key 
		FROM stores 
		WHERE id=$1`, storeID).
		Scan(&viewKey, &webhook.URL, &webhook.SecretKey)
	if err != nil {
		return errors.Wrap(err, "cannot query database")
	}

	// Create store wallet
	filename := fmt.Sprintf("%sstore_%d.wallet", config.WalletsPath, storeID)
	storeWallet, err := NewStoreWallet(filename, viewKey, webhook)
	if err != nil {
		return errors.Wrap(err, "cannot create new store wallet")
	}

	// Map store wallet to StoreID
	w.Map[storeID] = storeWallet
	return nil
}
