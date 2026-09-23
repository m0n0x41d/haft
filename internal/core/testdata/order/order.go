package orders

import "errors"

var ErrNotCancelable = errors.New("order cannot be cancelled")

type Order struct {
	Status string
	Total  int64
}

// Cancel changes an eligible order's state without changing its total.
func (o *Order) Cancel() error {
	if o == nil || !cancelable(o.Status) {
		return ErrNotCancelable
	}
	o.Status = "cancelled"
	return nil
}
