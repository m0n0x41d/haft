// Package delivery constructs bounded, lossless read views without IO. Domain
// outcomes are inputs; presentation never changes their success classification.
package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

const Format = "haft.api/2"
const Budget = 8192
const Guide = "Use haft.api/2. Default replies are summaries. Inspect delivery.complete, omissions and available; follow the supplied next_request or a part's request verbatim to read only needed detail. Parts and member directories are paged. Saved retained attachments expose verified decoded content parts. view=bytes restores exact bytes with digest/offset; it is for bulk clients, not routine model context. Read a complete claim (including extensions) before replacing it, and the full governing source body before assessing applicability. Stale means repeat the original query; expired means the disposable result was lost. A transient result is not saved evidence. Delivery completeness does not establish truth, attribution or current basis."

// Request is the executable read subset of the shared application request.
type Request struct {
	Format             string `json:"format"`
	Operation          string `json:"operation"`
	Ref                string `json:"ref"`
	View               string `json:"view,omitempty"`
	Part               string `json:"part,omitempty"`
	Cursor             string `json:"cursor,omitempty"`
	ExpectedDigest     string `json:"expected_digest,omitempty"`
	ExpectedGeneration string `json:"expected_generation,omitempty"`
	Limit              int    `json:"limit,omitempty"`
}
type Part struct {
	Name    string `json:"name"`
	Label   string `json:"label,omitempty"`
	Media   string `json:"media"`
	Raw     []byte `json:"raw"`
	Mutable bool   `json:"mutable,omitempty"`
}
type Document struct {
	Ref         string               `json:"ref"`
	Lifetime    string               `json:"lifetime"`
	Operation   string               `json:"operation"`
	Kind        string               `json:"result_kind"`
	IsError     bool                 `json:"is_error"`
	Basis       map[string]string    `json:"basis"`
	Coverage    string               `json:"coverage"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics"`
	Limits      []string             `json:"limits"`
	Summary     json.RawMessage      `json:"summary"`
	Parts       []Part               `json:"parts"`
	Unavailable string               `json:"continuation_unavailable,omitempty"`
	PageLimit   int                  `json:"page_limit,omitempty"`
}
type Descriptor struct {
	Name          string  `json:"part"`
	Label         string  `json:"label,omitempty"`
	LabelComplete bool    `json:"label_complete"`
	Media         string  `json:"media"`
	Bytes         int     `json:"total_bytes"`
	Digest        string  `json:"digest"`
	Request       Request `json:"request"`
}
type Omission struct {
	Parts       int    `json:"parts"`
	Diagnostics int    `json:"diagnostics"`
	Limits      int    `json:"limits"`
	Note        string `json:"note"`
}
type State struct {
	View          string       `json:"view"`
	Part          string       `json:"part"`
	Lifetime      string       `json:"lifetime"`
	Complete      bool         `json:"complete"`
	Encoding      string       `json:"encoding,omitempty"`
	Digest        string       `json:"digest,omitempty"`
	TotalBytes    int          `json:"total_bytes"`
	Offset        int          `json:"offset"`
	ReturnedBytes int          `json:"returned_bytes"`
	TotalItems    int          `json:"total_items,omitempty"`
	Next          *Request     `json:"next_request,omitempty"`
	Available     []Descriptor `json:"available,omitempty"`
	Catalog       *Request     `json:"parts_request,omitempty"`
	Omissions     Omission     `json:"omissions"`
	NoNext        string       `json:"no_next_reason,omitempty"`
	Budget        int          `json:"budget_bytes"`
}
type Response struct {
	Format      string               `json:"format"`
	Operation   string               `json:"operation"`
	Kind        string               `json:"result_kind"`
	Data        any                  `json:"data"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics"`
	Basis       map[string]string    `json:"basis"`
	Coverage    string               `json:"coverage"`
	Limits      []string             `json:"limits"`
	IsError     bool                 `json:"is_error"`
	Delivery    State                `json:"delivery"`
}

func (r Response) Failed() bool { return r.IsError }

func JSON(name string, v any, mutable bool) Part {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("non-JSON delivery input %s: %v", name, err))
	}
	return Part{Name: name, Media: "json", Raw: raw, Mutable: mutable}
}
func Text(name string, raw []byte) Part   { return Part{Name: name, Media: "text", Raw: raw} }
func Binary(name string, raw []byte) Part { return Part{Name: name, Media: "binary", Raw: raw} }
func Value(raw []byte) any {
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil
	}
	return v
}
func (d Document) part(name string) (Part, error) {
	if name == "summary" {
		return Part{Name: "summary", Media: "json", Raw: d.Summary}, nil
	}
	if name == "parts" {
		names := make([]string, len(d.Parts))
		for i, p := range d.Parts {
			names[i] = p.Name
		}
		// The index bytes identify the ordered names, while its generation binds
		// the current selection described by the directory (including relations).
		// Immutable member requests keep their independent content-only basis.
		return JSON("parts", names, true), nil
	}
	segments := strings.Split(name, "/")
	if len(segments) > 64 {
		return Part{}, fmt.Errorf("invalid_part")
	}
	var p Part
	found := false
	for _, candidate := range d.Parts {
		if candidate.Name == segments[0] {
			p = candidate
			found = true
			break
		}
	}
	if !found {
		return p, fmt.Errorf("missing_part")
	}
	for i := 1; i < len(segments); i++ {
		if p.Media != "json" {
			return p, fmt.Errorf("invalid_part")
		}
		members := children(p)
		n, err := strconv.Atoi(segments[i])
		if err != nil || n < 0 || n >= len(members) || strconv.Itoa(n) != segments[i] {
			return p, fmt.Errorf("invalid_part")
		}
		child := members[n]
		if i+1 < len(segments) && segments[i+1] == "key" {
			p = Text(child.part.Name+"/key", []byte(child.key))
			p.Mutable = child.part.Mutable
			i++
		} else {
			p = child.part
		}
	}
	return p, nil
}
func (d Document) Member(name string) (Part, error) { return d.part(name) }
