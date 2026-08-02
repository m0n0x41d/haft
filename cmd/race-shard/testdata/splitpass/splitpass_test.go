package splitpass

import (
	"fmt"
	"testing"
)

func TestParent(t *testing.T) {
	t.Run("alpha", func(t *testing.T) {})
	t.Run("beta", func(t *testing.T) {})
}

func Example_partition() {
	fmt.Println("partition")
	// Output: partition
}

func FuzzRoundTrip(f *testing.F) {
	f.Add("partition")
	f.Fuzz(func(t *testing.T, value string) {
		if got := fmt.Sprint(value); got != value {
			t.Fatalf("round trip = %q, want %q", got, value)
		}
	})
}
