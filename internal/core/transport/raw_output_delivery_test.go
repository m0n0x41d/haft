package transport

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/app"
	"github.com/m0n0x41d/haft/internal/core/check"
)

func TestMCPRawOutputDetailContinuations(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{"continuation80", bytes.Repeat([]byte{0x80}, 12000)},
		{"continuationBF", bytes.Repeat([]byte{0xbf}, 12000)},
		{"overlong", bytes.Repeat([]byte{0xc0, 0xaf}, 6000)},
		{"surrogate", bytes.Repeat([]byte{0xed, 0xa0, 0x80}, 4000)},
		{"aboveUnicode", bytes.Repeat([]byte{0xf4, 0x90, 0x80, 0x80}, 3000)},
		{"truncated", append(bytes.Repeat([]byte{'a'}, 12000), 0xe2, 0x82)},
		{"invalidAfterASCII", append([]byte("initial ASCII\n"), bytes.Repeat([]byte{0x80, 0xff, 0xfe, 0xc1, 0xbf}, 2400)...)},
		{"validMultibyte", []byte(strings.Repeat("Ж文🙂\\\"\n", 3000))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(t, app.Service{Root: t.TempDir()})
			exit := 1
			observed := deliveryCall(t, c, app.Request{Operation: "check", Action: "observe", Observation: &check.ObservationInput{Observed: check.Run{Started: true, ExitCode: &exit, Stdout: tc.raw, Stderr: tc.raw}}})
			if observed.Kind != "unattributable" || !observed.IsError {
				t.Fatal("invalid basis must remain unattributable", observed)
			}
			parts := deliveryParts(t, c, observed)
			encoding := "base64"
			if utf8.Valid(tc.raw) {
				encoding = "utf-8"
			}
			for _, name := range []string{"stdout", "stderr"} {
				q := parts[name].Request
				if q.View != "detail" || q.Cursor != "" {
					t.Fatal("expected advertised initial detail request")
				}
				first := readDelivery(t, c, q)
				if first.Delivery.Next == nil {
					t.Fatalf("raw output did not exercise an intermediate boundary: %s", first.Kind)
				}
				if got := deliveryBytes(t, c, q, observed.Kind, encoding, observed.IsError); !bytes.Equal(got, tc.raw) {
					t.Fatal("detail lost raw output")
				}
				q.View = "bytes"
				if got := deliveryBytes(t, c, q, observed.Kind, "base64", observed.IsError); !bytes.Equal(got, tc.raw) {
					t.Fatal("byte/detail parity")
				}
			}
		})
	}
}
