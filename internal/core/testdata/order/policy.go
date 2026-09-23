package orders

func cancelable(status string) bool {
	return status == "new" || status == "paid"
}

func Currency() string { return "USD" }
