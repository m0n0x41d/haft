package reservations

import (
	"errors"
	"math"
)

var ErrQuantity = errors.New("quantity cannot be reserved")

type Stock struct {
	Available uint64
	Reserved  uint64
}

func (s *Stock) Reserve(quantity uint64) error {
	if s == nil || !admitted(*s, quantity) {
		return ErrQuantity
	}
	s.Available -= quantity
	s.Reserved += quantity
	return nil
}

func admitted(s Stock, quantity uint64) bool {
	return quantity > 0 && quantity <= s.Available && s.Reserved <= math.MaxUint64-quantity
}
