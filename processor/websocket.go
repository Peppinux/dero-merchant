/*
	websocket.go manages communication to /pay/:payment_id/status WS connections
	listening for payment's status update.
	The only actual use of this file is to send the new status of a payment through WS
	to /pay/:payment_id helper pages a customer may be using.
*/

package processor

import (
	"net"

	"github.com/gobwas/ws/wsutil"
)

// ConnectionsToPayment is a type that maps a slice of web socket connections to a Payment ID
type ConnectionsToPayment map[string][]net.Conn

// PaymentWSConnections is the global variable that holds WS connections listening for payments' status update
var PaymentWSConnections = make(ConnectionsToPayment)

// SendStatusUpdate sends the new payment's status of paymentID to the WS connections listening for it
func (c ConnectionsToPayment) SendStatusUpdate(paymentID string, newStatus string) {
	for _, conn := range c[paymentID] {
		defer conn.Close()

		wsutil.WriteServerText(conn, []byte(newStatus))
	}

	delete(c, paymentID)
}
