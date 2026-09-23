package change

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"gopkg.in/yaml.v3"
)

func diag(code, field, msg string) carrier.Diagnostic {
	return carrier.Diagnostic{Code: code, Path: field, Message: msg, Severity: "error"}
}
func ValidID(id string) bool {
	return strings.HasPrefix(id, "chg-") && carrier.ValidID("note-"+strings.TrimPrefix(id, "chg-"))
}
func ParseRef(s string) (string, string, error) {
	id, hash, ok := strings.Cut(s, "@")
	if !ok || !ValidID(id) || !carrier.ValidDigest(hash) {
		return "", "", fmt.Errorf("change ref requires chg ID and full snapshot digest")
	}
	return id, hash, nil
}
func Parse(raw []byte) Document {
	d := Document{Raw: bytes.Clone(raw)}
	front, body, err := carrier.SplitFrontmatter(raw)
	d.Body = bytes.Clone(body)
	if err != nil {
		d.Diagnostics = []carrier.Diagnostic{diag("frontmatter", "", err.Error())}
		return d
	}
	if !utf8.Valid(raw) {
		d.Diagnostics = []carrier.Diagnostic{diag("invalid_utf8", "", "Change is not UTF-8; original bytes retained")}
		return d
	}
	node, ds := carrier.ParseYAML(front)
	d.Diagnostics = ds
	if carrier.HasErrors(ds) {
		return d
	}
	if err := node.Decode(&d.Change); err != nil {
		d.Diagnostics = append(ds, diag("invalid_fields", "", err.Error()))
		return d
	}
	d.Diagnostics = append(d.Diagnostics, Validate(d.Change)...)
	return d
}
func Encode(c Change, body []byte) (raw []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			raw = nil
			err = fmt.Errorf("encode change: %v", p)
		}
	}()
	front, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	return carrier.JoinFrontmatter(front, body), nil
}

// Patch claims distinguish omitted examples (preserve) from an explicit empty
// sequence (replacement). Carrier fields normally omit empty collections, so
// the change wire must retain that distinction in both supported encodings.
func (o Operation) MarshalYAML() (any, error) {
	type plain Operation
	var node yaml.Node
	if err := node.Encode(plain(o)); err != nil {
		return nil, err
	}
	if o.Claim != nil && o.Claim.Examples != nil && len(o.Claim.Examples) == 0 {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "claim" {
				claim := node.Content[i+1]
				claim.Content = append(claim.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "examples"},
					&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{}},
				)
			}
		}
	}
	return &node, nil
}

func (o Operation) MarshalJSON() ([]byte, error) {
	type plain Operation
	raw, err := json.Marshal(plain(o))
	if err != nil || o.Claim == nil || o.Claim.Examples == nil || len(o.Claim.Examples) != 0 {
		return raw, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	var claim map[string]json.RawMessage
	if err := json.Unmarshal(fields["claim"], &claim); err != nil {
		return nil, err
	}
	claim["examples"] = json.RawMessage("[]")
	fields["claim"], err = json.Marshal(claim)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}
func Validate(c Change) []carrier.Diagnostic {
	var ds []carrier.Diagnostic
	bad := func(code, field, msg string) { ds = append(ds, diag(code, field, msg)) }
	if c.Format != Format {
		bad("unsupported_format", "format", "Unsupported change format")
		return ds
	}
	if !ValidID(c.ID) || !ValidID(c.ChangeKey) {
		bad("invalid_id", "id", "Change ID/key must be chg-date-eight-hex")
	}
	if c.State != "open" && c.State != "archived" {
		bad("invalid_state", "state", "Change state is open or archived, never a check/acceptance result")
	}
	if strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.Intent) == "" {
		bad("required", "intent", "Title and change intent are required")
	}
	if _, err := time.Parse(time.RFC3339, c.CreatedAt); err != nil {
		bad("invalid_time", "created_at", "Timestamp must be RFC3339")
	}
	if len(c.Supersedes) == 0 && c.ChangeKey != c.ID {
		bad("invalid_change_key", "change_key", "Initial change key must be its own ID")
	}
	if len(c.Supersedes) > 0 && strings.TrimSpace(c.SupersedeReason) == "" {
		bad("required", "supersede_reason", "Change revision needs a reason")
	}
	seen := map[string]bool{}
	for _, ref := range c.Supersedes {
		id, _, err := ParseRef(ref)
		if err != nil || id == c.ID || seen[ref] {
			bad("invalid_predecessor", "supersedes", "Predecessors must be distinct pinned other changes")
		}
		seen[ref] = true
	}
	if c.WriteReceipt != nil && (c.WriteReceipt.RequestID == "" || !carrier.ValidDigest(c.WriteReceipt.PayloadDigest)) {
		bad("invalid_receipt", "write_receipt", "Receipt requires request ID and full payload digest")
	}
	if len(c.Patches) == 0 && strings.TrimSpace(c.NoSpecChangeReason) == "" {
		bad("required", "no_spec_change_reason", "A change with no spec delta must explain why")
	}
	for _, ref := range c.Rationale {
		if _, err := carrier.ParseRef(ref); err != nil {
			bad("invalid_rationale_ref", "rationale", err.Error())
		}
	}
	for _, ref := range c.Evidence {
		r, err := carrier.ParseRef(ref)
		if err != nil || !r.Pinned() || !strings.HasPrefix(r.RecordID, "ev-") {
			bad("invalid_evidence_ref", "evidence", "Evidence references pin exact evidence records or uses")
		}
	}
	for _, ref := range c.Checks {
		k, v, ok := strings.Cut(ref, ":")
		if !ok || v == "" || !strings.Contains("|test|pbt|type|contract|lint|arch|gate|model|manual|", "|"+k+"|") {
			bad("invalid_check", "checks", "Check requires a supported selector kind")
		}
	}
	tasks := map[string]bool{}
	for _, t := range c.Tasks {
		if t.ID == "" || tasks[t.ID] || strings.TrimSpace(t.Text) == "" {
			bad("invalid_task", "tasks", "Tasks need unique IDs and readable text")
		}
		tasks[t.ID] = true
	}
	seen = map[string]bool{}
	for i, p := range c.Patches {
		field := fmt.Sprintf("patches[%d]", i)
		r, err := carrier.ParseRef(p.Base)
		if err != nil || !r.Pinned() || r.ClaimID != "" || !strings.HasPrefix(r.RecordID, "spec-") {
			bad("invalid_base", field+".base", "Authored base pins an exact whole spec edition")
		}
		if seen[p.Base] {
			bad("duplicate_base", field, "One ordered operation list per section")
		}
		seen[p.Base] = true
		if len(p.Operations) == 0 && p.Body == nil {
			bad("empty_patch", field, "Section patch has no operations or explicit body replacement")
		}
		if p.Body == nil && (p.ExpectedBodyDigest != "" || p.BodyChangeReason != "") {
			bad("invalid_body_change", field, "Body precondition/reason require an explicit replacement")
		}
		if p.Body != nil && (!carrier.ValidDigest(p.ExpectedBodyDigest) || strings.TrimSpace(p.BodyChangeReason) == "") {
			bad("unacknowledged_body_change", field, "Body replacement requires exact prior body digest and reason")
		}
		for j, o := range p.Operations {
			f := fmt.Sprintf("%s.operations[%d]", field, j)
			switch o.Op {
			case "ADDED":
				if o.Claim == nil || o.Claim.ID == "" || (o.ClaimID != "" && o.ClaimID != o.Claim.ID) {
					bad("invalid_operation", f, "ADDED supplies one claim with its new ID")
				}
			case "MODIFIED":
				if o.Claim == nil || o.ClaimID == "" || o.Claim.ID != o.ClaimID {
					bad("invalid_operation", f, "MODIFIED preserves the addressed claim ID")
				}
			case "REMOVED":
				if o.ClaimID == "" || o.Claim != nil || o.NewID != "" {
					bad("invalid_operation", f, "REMOVED addresses only an existing claim")
				}
			case "RENAMED":
				if o.ClaimID == "" || o.NewID == "" || o.ClaimID == o.NewID || o.Claim != nil {
					bad("invalid_operation", f, "RENAMED names an old and different new ID")
				}
			default:
				bad("unsupported_operation", f, "Unknown operation has no patch semantics")
			}
			if (o.Op == "ADDED" || o.Op == "MODIFIED") && o.NewID != "" {
				bad("invalid_operation", f, "Only RENAMED may supply new_id")
			}
			if o.Op != "ADDED" && strings.TrimSpace(o.Reason) == "" {
				bad("required", f+".reason", "A changed or removed claim requires a readable reason")
			}
			if (len(o.RemoveExamples) > 0 || len(o.RemoveFields) > 0) && o.Op != "MODIFIED" {
				bad("invalid_operation", f, "Explicit field/example removal belongs only to MODIFIED")
			}
		}
	}
	return ds
}
