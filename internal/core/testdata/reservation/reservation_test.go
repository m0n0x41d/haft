package reservations

import (
	"math"
	"math/rand"
	"testing"
	"testing/quick"
)

func TestReservationConservesStock(t *testing.T) {
	property := func(available, reserved, requested uint32) bool {
		s := Stock{uint64(available), uint64(reserved)}
		before := s
		quantity := uint64(requested)
		err := s.Reserve(quantity)
		if quantity == 0 || quantity > before.Available {
			return err == ErrQuantity && s == before
		}
		return err == nil && s.Available == before.Available-quantity && s.Reserved == before.Reserved+quantity && s.Available+s.Reserved == before.Available+before.Reserved
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(24))}); err != nil {
		t.Fatal(err)
	}
}

func TestReservationRejectsWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		stock    Stock
		quantity uint64
	}{{Stock{5, 2}, 0}, {Stock{5, 2}, 6}, {Stock{5, math.MaxUint64}, 1}} {
		before := test.stock
		if err := test.stock.Reserve(test.quantity); err != ErrQuantity || test.stock != before {
			t.Fatalf("rejected reservation mutated stock: %+v", test)
		}
	}
	var missing *Stock
	if missing.Reserve(1) != ErrQuantity {
		t.Fatal("nil stock admitted")
	}
}
