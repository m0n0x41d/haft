package fail

import "testing"

func TestDeliberateFailure(t *testing.T) {
	t.Fatal("deliberate race-shard fixture failure")
}
