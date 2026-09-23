package orders

import (
	"math/rand"
	"testing"
	"testing/quick"
)

// This oracle covers direct domain transitions, not HTTP or persistence.
func TestCancelPreservesTotal(t *testing.T) {
	property := func(total int64, paid bool) bool {
		state := "new"
		if paid {
			state = "paid"
		}
		o := Order{Status: state, Total: total}
		err := o.Cancel()
		return err == nil && o.Status == "cancelled" && o.Total == total
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(23))}); err != nil {
		t.Fatal(err)
	}
}

func TestCancelStates(t *testing.T) {
	for _, state := range []string{"new", "paid", "shipped", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			o := Order{Status: state, Total: 123}
			before := o
			err := o.Cancel()
			if state == "new" || state == "paid" {
				if err != nil || o.Status != "cancelled" || o.Total != before.Total {
					t.Fatalf("successful cancellation: got %+v, error %v", o, err)
				}
			} else if err != ErrNotCancelable || o != before {
				t.Fatalf("rejected cancellation changed order: got %+v, error %v", o, err)
			}
		})
	}
	var absent *Order
	if absent.Cancel() != ErrNotCancelable {
		t.Fatal("nil order must be rejected")
	}
}

// A green result here cannot support cancellation claims.
func TestCurrency(t *testing.T) {
	if Currency() != "USD" {
		t.Fatal("unexpected currency")
	}
}

func TestExternalSettlement(t *testing.T) {
	t.Skip("external integration deliberately outside this fixture")
}
