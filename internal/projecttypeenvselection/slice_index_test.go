package projecttypeenvselection

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestCanonicalReadersRejectMaxUint64SliceCoordinates(t *testing.T) {
	hostile := make([]byte, 8)
	binary.BigEndian.PutUint64(hostile, math.MaxUint64)

	tests := []struct {
		name string
		read func() error
	}{
		{
			name: "graph snapshot string length",
			read: func() error {
				reader := graphSnapshotReader{value: hostile}
				_, err := reader.readString("project")
				return err
			},
		},
		{
			name: "Stage count",
			read: func() error {
				reader := stageReader{value: hostile}
				_, err := reader.readCount("ordered E DAG", maximumStageExtensions)
				return err
			},
		},
		{
			name: "Stage byte length",
			read: func() error {
				reader := stageReader{value: hostile}
				_, err := reader.readBytes("coordinate", maximumStageCoordinateBytes)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.read()
			if err == nil || !strings.Contains(err.Error(), "does not fit this runtime") {
				t.Fatalf("hostile MaxUint64 coordinate error = %v", err)
			}
		})
	}
}
