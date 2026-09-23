package carrier

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var prefixes = map[string]string{"decision": "dec", "spec": "spec", "note": "note", "problem": "prob", "evidence": "ev", "options": "opt"}
var checkKinds = map[string]bool{"test": true, "pbt": true, "type": true, "contract": true, "lint": true, "arch": true, "gate": true, "model": true, "manual": true}
var commitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// Validate checks carrier structure, never evidence truth or harness fitness.
// Strict additionally requires law/guard validation basis or honest unchecked.
func Validate(r Record, strict bool) []Diagnostic {
	var ds []Diagnostic
	bad := func(code, path, message string) { ds = append(ds, diagnostic(code, path, message)) }
	required := func(path, value string) {
		if strings.TrimSpace(value) == "" {
			bad("required", path, "Required field is empty")
		}
	}
	if r.Format != "haft/1" {
		bad("unsupported_format", "format", "Unknown format is readable but has no semantic projection")
		return ds
	}
	prefix, known := prefixes[r.Kind]
	if !known {
		bad("unsupported_kind", "kind", "Unknown kind has no semantic projection")
	}
	if !ValidID(r.ID) || (known && !strings.HasPrefix(r.ID, prefix+"-")) {
		bad("invalid_id", "id", "Record ID must have its kind prefix, a calendar date and eight lowercase hex digits")
	}
	required("title", r.Title)
	if r.Status != "active" && r.Status != "proposed" {
		bad("invalid_status", "status", "Authored status must be proposed or active")
	}
	if r.Origin != "agent_proposal" && r.Origin != "operator_request" && r.Origin != "migrated_9x" {
		bad("invalid_origin", "origin", "Unknown record origin")
	}
	if !validAbout(r.About) {
		bad("invalid_about", "about", "Subject must be an explicit supported project-local address")
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
		bad("invalid_time", "created_at", "created_at must be RFC3339")
	}
	if r.UpdatedAt != "" {
		if _, err := time.Parse(time.RFC3339, r.UpdatedAt); err != nil {
			bad("invalid_time", "updated_at", "updated_at must be RFC3339")
		}
	}
	if r.OperatorConfirmed && r.Origin != "operator_request" {
		bad("invalid_confirmation", "operator_confirmed", "Confirmation requires honest operator_request origin")
	}
	if (r.Kind == "decision" || r.Kind == "spec") && r.Status == "active" && r.Origin != "migrated_9x" && (!r.OperatorConfirmed || r.Origin != "operator_request") {
		bad("operator_confirmation_required", "operator_confirmed", "Active decision/spec requires an operator_request and explicit confirmation")
	}
	if _, exists := r.Extra["superseded_by"]; exists {
		bad("writable_reverse_relation", "superseded_by", "superseded_by is derived, never writable")
	}
	if r.WriteReceipt != nil {
		required("write_receipt.request_id", r.WriteReceipt.RequestID)
		if !ValidDigest(r.WriteReceipt.PayloadDigest) {
			bad("invalid_digest", "write_receipt.payload_digest", "Full SHA-256 required")
		}
	}
	seenRefs := map[string]bool{}
	for i, s := range r.Supersedes {
		p := fmt.Sprintf("supersedes[%d]", i)
		ref, err := ParseRef(s)
		if err != nil || !ref.Pinned() || ref.ClaimID != "" {
			bad("invalid_predecessor", p, "Predecessor must pin a whole record edition")
		}
		if ref.RecordID == r.ID {
			bad("succession_cycle", p, "A record cannot supersede itself")
		}
		if seenRefs[s] {
			bad("duplicate_ref", p, "Duplicate predecessor")
		}
		seenRefs[s] = true
	}
	if len(r.Supersedes) > 0 {
		required("supersede_reason", r.SupersedeReason)
	}
	for i, s := range r.Constrains {
		if r.Kind != "spec" && !(r.Kind == "decision" && r.Disposition == "choose_now") {
			bad("constraint_kind", "constrains", "Only spec and choose_now decision may constrain")
		}
		if err := ValidateSelector(s); err != nil {
			bad("invalid_selector", fmt.Sprintf("constrains[%d]", i), err.Error())
		}
	}
	for i, l := range r.Links {
		p := fmt.Sprintf("links[%d]", i)
		if l.Kind != "relates_to" && l.Kind != "relies_on" {
			bad("invalid_relation", p+".kind", "Links support relates_to or relies_on; other relations have canonical fields")
		}
		if l.Kind == "relies_on" {
			if _, err := ParseRef(l.Target); err != nil {
				bad("invalid_ref", p+".target", err.Error())
			}
			required(p+".reason", l.Reason)
		} else if !validAbout(l.Target) {
			bad("invalid_ref", p+".target", "Unrecognized navigation target")
		}
	}
	if r.OptionsRef != "" {
		ref, err := ParseRef(r.OptionsRef)
		if err != nil || !ref.Pinned() || !strings.HasPrefix(ref.RecordID, "opt-") || ref.ClaimID != "" {
			bad("invalid_options_ref", "options_ref", "Selected options require an exact options snapshot")
		}
	}
	for i, s := range r.Sources {
		ds = append(ds, validateSource(s, fmt.Sprintf("sources[%d]", i))...)
	}
	switch r.Kind {
	case "decision":
		required("object", r.Object)
		required("question", r.Question)
		required("why", r.Why)
		switch r.Disposition {
		case "choose_now":
			required("chosen", r.Chosen)
			rejected := false
			for _, o := range r.Options {
				if o.ID == r.Chosen && o.Verdict == "rejected" {
					bad("chosen_option_rejected", "options", "The chosen option cannot also be a rejected alternative")
				}
				if o.ID != r.Chosen && o.Verdict == "rejected" && strings.TrimSpace(o.Reason) != "" {
					rejected = true
				}
			}
			if !rejected && strings.TrimSpace(r.NoAlternativeReason) == "" {
				bad("alternative_basis_required", "options", "A rejected alternative with reason or no_alternative_reason is required")
			}
		case "reject_current_set":
			if len(r.Options) == 0 {
				bad("rejected_set_required", "options", "Preserve the rejected option set")
			}
			for i, o := range r.Options {
				if o.Verdict != "rejected" || strings.TrimSpace(o.Reason) == "" {
					bad("invalid_rejected_option", fmt.Sprintf("options[%d]", i), "Rejected options require verdict and reason")
				}
			}
		case "probe_again":
			required("probe", r.Probe)
		case "reroute":
			required("reroute_to", r.RerouteTo)
		default:
			bad("invalid_disposition", "disposition", "Unknown decision disposition")
		}
		if r.Disposition != "choose_now" && r.Chosen != "" {
			bad("disposition_shape", "chosen", "Only choose_now contains a chosen option")
		}
	case "problem":
		required("object", r.Object)
		required("question", r.Question)
	case "options":
		required("question", r.Question)
		if len(r.Options) == 0 {
			bad("options_required", "options", "An option set cannot be empty")
		}
		if r.NextUse != "probe_again" && r.NextUse != "shortlist" && r.NextUse != "reopen_replay" {
			bad("invalid_next_use", "next_use", "Unknown next use")
		}
		if r.NextUse == "probe_again" {
			required("probe", r.Probe)
		}
		if r.Comparison != nil && len(r.Comparison.NonDominated) > 0 {
			required("comparison.basis", r.Comparison.Basis)
			required("comparison.comparator", r.Comparison.Comparator)
		}
	case "spec":
		if !slugPattern.MatchString(r.Slug) {
			bad("invalid_slug", "slug", "Spec requires a stable slug")
		}
		required("receiving_use", r.ReceivingUse)
		if len(r.Claims) == 0 {
			ds = append(ds, Diagnostic{Code: "validation_basis_absent", Path: "claims", Message: "No claims or validation basis are declared; this is not a checked specification", Severity: "warning"})
		}
		for i, a := range r.Aliases {
			if !slugPattern.MatchString(a) {
				bad("invalid_alias", fmt.Sprintf("aliases[%d]", i), "Alias must be a spec slug")
			}
		}
		if r.Retirement != nil {
			required("retirement.reason", r.Retirement.Reason)
			if len(r.Claims) > 0 {
				bad("retirement_shape", "claims", "Retired section cannot contain live claims")
			}
		}
		ds = append(ds, validateClaims(r, strict)...)
	case "evidence":
		required("claim", r.Claim)
		required("method", r.Method)
		required("source", r.Source)
		if _, err := time.Parse(time.RFC3339, r.ObservedAt); err != nil {
			bad("invalid_time", "observed_at", "observed_at must be RFC3339")
		}
		if r.Basis == nil {
			bad("basis_required", "basis", "Observation requires its exact basis")
		} else {
			if !oneOf(r.Basis.Kind, "code", "data", "report", "config", "interview", "period", "other") {
				bad("invalid_basis", "basis.kind", "Unknown evidence basis kind")
			}
			required("basis.ref", r.Basis.Ref)
		}
		ids := map[string]bool{}
		for i, u := range r.Uses {
			p := fmt.Sprintf("uses[%d]", i)
			if !localIDPattern.MatchString(u.ID) || ids[u.ID] {
				bad("invalid_use_id", p+".id", "Use ID must be valid and unique")
			}
			ids[u.ID] = true
			ref, err := ParseRef(u.Target)
			if err != nil || !ref.Pinned() {
				bad("unpinned_evidence_target", p+".target", "Published evidence must target an exact snapshot")
			}
			if !oneOf(u.Polarity, "supports", "weakens", "inconclusive") {
				bad("invalid_polarity", p+".polarity", "Unknown evidence use polarity")
			}
			required(p+".scope", u.Scope)
			if u.Disposition != "" {
				required(p+".receiving_use", u.ReceivingUse)
			}
		}
	}
	options := map[string]bool{}
	for i, o := range r.Options {
		if !localIDPattern.MatchString(o.ID) || options[o.ID] {
			bad("invalid_option_id", fmt.Sprintf("options[%d].id", i), "Option ID must be valid and unique")
		}
		options[o.ID] = true
	}
	if r.Comparison != nil {
		for _, id := range r.Comparison.NonDominated {
			if !options[id] {
				bad("unresolved_option", "comparison.non_dominated", "Non-dominated member is absent from option set")
			}
		}
	}
	return ds
}

func validateClaims(r Record, strict bool) []Diagnostic {
	var ds []Diagnostic
	bad := func(code, p, msg string) { ds = append(ds, diagnostic(code, p, msg)) }
	claims := map[string]Claim{}
	retired := map[string]bool{}
	for _, id := range r.RetiredClaimIDs {
		if !localIDPattern.MatchString(id) || retired[id] {
			bad("invalid_retired_claim_id", "retired_claim_ids", "Retired claim IDs must be valid and unique")
		}
		retired[id] = true
	}
	for i, c := range r.Claims {
		p := fmt.Sprintf("claims[%d]", i)
		_, dup := claims[c.ID]
		if !localIDPattern.MatchString(c.ID) || dup || retired[c.ID] {
			bad("invalid_claim_id", p+".id", "Claim ID must be valid, unique and not retired")
		}
		claims[c.ID] = c
		if Quadrant(c.Kind) == "" {
			bad("invalid_claim_kind", p+".kind", "Unknown claim kind")
		}
		if strings.TrimSpace(c.Text) == "" {
			bad("required", p+".text", "Claim text is required")
		}
		if (c.Kind == "law" || c.Kind == "guard") && len(c.Checks) == 0 && strings.TrimSpace(c.Unchecked) == "" {
			d := Diagnostic{Code: "validation_basis_absent", Path: p, Message: "Declare checks or an unchecked reason; structure does not prove harness fitness", Severity: "warning"}
			if strict {
				d.Severity = "error"
			}
			ds = append(ds, d)
		}
		for j, b := range c.Checks {
			q := fmt.Sprintf("%s.checks[%d]", p, j)
			k, v, ok := strings.Cut(b.Ref, ":")
			if !ok || !checkKinds[k] || strings.TrimSpace(v) == "" {
				bad("invalid_check", q+".ref", "Check requires a supported selector kind")
			}
			if strings.TrimSpace(b.Covers) == "" {
				bad("required", q+".covers", "Checking scope and limits are required")
			}
		}
		for j, b := range c.ImplementedBy {
			q := fmt.Sprintf("%s.implemented_by[%d]", p, j)
			if err := ValidateSelector(b.Ref); err != nil {
				bad("invalid_selector", q+".ref", err.Error())
			}
			if strings.TrimSpace(b.Covers) == "" {
				bad("required", q+".covers", "Implemented portion and limits are required")
			}
		}
		examples := map[string]bool{}
		for j, e := range c.Examples {
			if !localIDPattern.MatchString(e.ID) || examples[e.ID] {
				bad("invalid_example_id", fmt.Sprintf("%s.examples[%d].id", p, j), "Example ID must be valid and unique")
			}
			examples[e.ID] = true
		}
		for j, e := range c.EvidenceInputs {
			q := fmt.Sprintf("%s.evidence_inputs[%d]", p, j)
			if c.Kind != "guard" {
				bad("quadrant_dependency", q, "Only a guard may consume separately established evidence")
			}
			ref, err := ParseRef(e.Ref)
			if err != nil || !ref.Pinned() || !strings.HasPrefix(ref.RecordID, "ev-") || ref.ClaimID == "" {
				bad("invalid_evidence_input", q+".ref", "Evidence input pins a concrete evidence use")
			}
			if strings.TrimSpace(e.Applicability) == "" {
				bad("required", q+".applicability", "Evidence input applicability is required")
			}
		}
	}
	for i, c := range r.Claims {
		for j, s := range c.Refs {
			p := fmt.Sprintf("claims[%d].refs[%d]", i, j)
			if target, ok := claims[s]; ok {
				if !AllowedDependency(c.Kind, target.Kind) {
					bad("quadrant_dependency", p, "Dependency violates L→A/D/E or A→D separation")
				}
				continue
			}
			if _, err := ParseRef(s); err != nil {
				bad("unresolved_claim", p, "Neither a local claim ID nor a valid external claim address")
			} else {
				ref, _ := ParseRef(s)
				if ref.ClaimID == "" {
					bad("invalid_claim_ref", p, "External dependency requires a claim ID")
				}
			}
		}
	}
	return ds
}
func Quadrant(kind string) string {
	switch kind {
	case "definition", "law":
		return "L"
	case "guard":
		return "A"
	case "prescription":
		return "D"
	}
	return ""
}
func AllowedDependency(from, to string) bool {
	f, t := Quadrant(from), Quadrant(to)
	return f != "" && t != "" && (f == "D" || f == t || f == "A" && t == "L")
}
func oneOf(value string, options ...string) bool {
	for _, s := range options {
		if value == s {
			return true
		}
	}
	return false
}

func validateSource(s Source, p string) []Diagnostic {
	var ds []Diagnostic
	bad := func(code, field, msg string) { ds = append(ds, diagnostic(code, p+"."+field, msg)) }
	if strings.TrimSpace(s.Ref) == "" {
		bad("required", "ref", "Source locator is required")
	}
	r := s.SourceRevision
	switch r.Kind {
	case "git":
		if r.Repository == "" || !commitPattern.MatchString(r.Commit) || r.TreeDigest != "" {
			bad("invalid_source_revision", "source_revision", "Git source needs repository and full actual source commit")
		}
	case "local_tree":
		if r.Repository == "" || !ValidDigest(r.TreeDigest) || r.Commit != "" {
			bad("invalid_source_revision", "source_revision", "Local source needs repository and tree digest, not a parent Git commit")
		}
	case "unknown":
		if strings.TrimSpace(r.Reason) == "" || r.Commit != "" || r.TreeDigest != "" {
			bad("invalid_source_revision", "source_revision", "Unknown source needs reason and cannot invent an exact revision")
		}
	default:
		bad("invalid_source_revision", "source_revision.kind", "Unknown source revision kind")
	}
	if r.Kind == "git" || r.Kind == "local_tree" {
		if s.PublicationPath == "" {
			bad("required", "publication_path", "Recovered source requires its publication path")
		}
		if !ValidDigest(s.BodyDigest) {
			bad("invalid_digest", "body_digest", "Recovered source requires full body digest")
		}
		if len(s.Lines) != 2 || s.Lines[0] < 1 || s.Lines[1] < s.Lines[0] {
			bad("invalid_lines", "lines", "Source requires inclusive start/end line bounds")
		}
		if !ValidDigest(s.SnapshotRef) {
			bad("invalid_digest", "snapshot_ref", "Recovered source requires a portable source snapshot digest")
		}
	}
	return ds
}
