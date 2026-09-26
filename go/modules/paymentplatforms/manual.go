package paymentplatforms

import (
	"context"
	"fmt"

	"tls-rest/go/engine/controllers/functions"
)

// manual collects nothing online: the payment stays pending until an admin marks it paid; config.instructions is shown to the payer.
type manual struct{}

func (manual) ID() string   { return "manual" }
func (manual) Name() string { return "Manual (bank transfer / cash)" }

func (manual) Charge(_ context.Context, inst Instance, c Charge) (Result, error) {
	return Result{
		ExternalID: fmt.Sprintf("manual-%d", c.PaymentID),
		Status:     "pending",
		Message:    functions.Coerce[string](inst.Config["instructions"]),
	}, nil
}

func init() { RegisterProvider(manual{}) }
