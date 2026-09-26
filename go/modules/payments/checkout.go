package payments

import (
	"context"
	"errors"
	"fmt"
	"math"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/modules/paymentplatforms"
)

const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusRefunded  = "refunded"
)

// MaxQuantity caps one line's quantity, in a cart and in a single payment.
const MaxQuantity = 100

// Item is one purchased line, snapshotted into the payment so later price changes don't rewrite history.
type Item struct {
	ProductID int64   `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Currency  string  `json:"currency"`
}

// Outcome is what the payer gets back from Checkout.
type Outcome struct {
	PaymentID   int64  `json:"payment_id"`
	Status      string `json:"status"`
	RedirectURL string `json:"redirect_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Checkout records a pending payment for items and charges it through the platform instance.
func Checkout(ctx context.Context, userID int, platformID int64, items []Item) (Outcome, error) {
	if len(items) == 0 {
		return Outcome{}, errors.New("nothing to pay for")
	}
	currency := items[0].Currency
	total := 0.0
	for _, it := range items {
		if it.Quantity <= 0 || it.Quantity > MaxQuantity {
			return Outcome{}, fmt.Errorf("quantity must be between 1 and %d", MaxQuantity)
		}
		if it.Currency != currency {
			return Outcome{}, errors.New("items use different currencies; pay for them separately")
		}
		total += it.Price * float64(it.Quantity)
	}
	total = math.Round(total*100) / 100

	inst, err := paymentplatforms.Load(ctx, platformID)
	if err != nil {
		return Outcome{}, err
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return Outcome{}, err
	}
	id, err := db.InsertRow("payments", map[string]interface{}{
		"user_id":     userID,
		"platform_id": platformID,
		"amount":      total,
		"currency":    currency,
		"status":      StatusPending,
		"items":       items,
		"created_by":  userID,
	})
	if err != nil {
		return Outcome{}, err
	}

	res, err := paymentplatforms.Run(ctx, inst, paymentplatforms.Charge{
		PaymentID:   id,
		Amount:      total,
		Currency:    currency,
		Description: fmt.Sprintf("Payment #%d", id),
	})
	if err != nil {
		_, _ = db.UpdateRow("payments", map[string]interface{}{"status": StatusFailed}, "id", id)
		return Outcome{PaymentID: id, Status: StatusFailed}, err
	}

	status := res.Status
	if status == "" {
		status = StatusPending
	}
	if _, err := db.UpdateRow("payments", map[string]interface{}{
		"status":      status,
		"external_id": res.ExternalID,
		"details":     res,
	}, "id", id); err != nil {
		return Outcome{PaymentID: id, Status: status}, err
	}
	return Outcome{PaymentID: id, Status: status, RedirectURL: res.RedirectURL, Message: res.Message}, nil
}
