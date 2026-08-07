package splitfail

import "testing"

func TestParent(t *testing.T) {
	t.Run("deliberate-failure", func(t *testing.T) {
		t.Fatal("deliberate split-package fixture failure")
	})
}
