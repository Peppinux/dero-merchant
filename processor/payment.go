package processor

import (
	"sync"
	"time"
)

// Payment statuses
const (
	PaymentStatusPending = "pending"
	PaymentStatusPaid    = "paid"
	PaymentStatusExpired = "expired"
	PaymentStatusError   = "error"
)

// PendingPayment represents a pending payment
type PendingPayment struct {
	AtomicDeroAmount uint64
	CreationTime     time.Time
}

// NewPendingPayment returns a new PendingPayment struct
func NewPendingPayment(atomicDeroAmount uint64) *PendingPayment {
	return &PendingPayment{
		AtomicDeroAmount: atomicDeroAmount,
		CreationTime:     time.Now(),
	}
}

// MinutesFromCreation returns the number of minutes passed from the creation of the pending payment
func (p *PendingPayment) MinutesFromCreation() float64 {
	t := time.Now().Sub(p.CreationTime)
	return t.Minutes()
}

// PendingPayments stores a map of PendingPayment(s) associated to their payment ID,
// a RWMutex for map synchronization
// and a ticker needed to loop over PendingPayments every minute to check if payment was sent to the wallet.
type PendingPayments struct {
	Map     map[string]*PendingPayment
	Mutex   sync.RWMutex
	Checker *time.Ticker
}

// NewPendingPayments returns a new PendingPayments struct
func NewPendingPayments() *PendingPayments {
	return &PendingPayments{
		Map: make(map[string]*PendingPayment),
	}
}

// Set adds a new PendingPayment associated to its PaymentID to a PendingPayments struct
func (p *PendingPayments) Set(paymentID string, pendingPayment *PendingPayment) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	p.Map[paymentID] = pendingPayment
}

// Delete deletes a PendingPayment associated to a PaymentID from a PendingPayments struct
func (p *PendingPayments) Delete(paymentID string) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	delete(p.Map, paymentID)
}

// Count returns the number of PendingPayment(s) in a PendingPayments struct
func (p *PendingPayments) Count() int {
	p.Mutex.RLock()
	defer p.Mutex.RUnlock()

	return len(p.Map)
}

// StartChecker starts the ticker that a wallet uses to loop over its PendingPayments every minute until all are paid or expired.
func (p *PendingPayments) StartChecker() {
	p.Checker = time.NewTicker(time.Minute)
}
