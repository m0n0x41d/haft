package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/delivery"
)

func object(v any) map[string]any   { m, _ := v.(map[string]any); return m }
func str(v any) string              { s, _ := v.(string); return s }
func rawJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func selectFields(v map[string]any, keys ...string) map[string]any {
	m := map[string]any{}
	for _, k := range keys {
		if x, ok := v[k]; ok {
			m[k] = x
		}
	}
	return m
}
func makeDelivery(q Request, r Result) delivery.Document {
	if _, err := json.Marshal(r); err != nil {
		return yamlDelivery(q, r, err)
	}
	d := delivery.Document{Operation: r.Operation, Kind: r.Kind, IsError: r.Failed(), Basis: r.Basis, Coverage: r.Coverage, Diagnostics: r.Diagnostics, Limits: r.Limits, Lifetime: "transient_cache", PageLimit: q.Limit}
	// Normalize through the public domain codec, including inline authored
	// extensions. UseNumber preserves exact large extension integers on reads.
	data := object(delivery.Value(rawJSON(r.Data)))
	summary := selectFields(data, "exact_ref", "current_basis", "declared_check_binding", "code_complete", "runner_started", "command", "run_environment", "records", "checks_executed", "strict", "kind", "paths", "generation", "transaction_id", "digest", "advisory", "total_matches", "truncated")
	add := func(name string, v any, mutable bool) { d.Parts = append(d.Parts, delivery.JSON(name, v, mutable)) }
	diagnostics := delivery.JSON("diagnostics", r.Diagnostics, false)
	// Every result has full diagnostics, metadata and lossless original result.
	// Domain-specific parts make routine reading independent of that raw result.
	if q.Operation == "recall" && q.Ref != "" && data["resolution"] != nil {
		found := object(data["resolution"])
		doc := object(found["document"])
		record := object(doc["record"])
		summary = selectFields(record, "id", "kind", "title", "status", "origin", "about", "claim", "observed_at", "basis", "uses")
		summary["exact_ref"] = data["exact_ref"]
		summary["snapshot_persisted"] = data["snapshot_persisted"]
		summary["current_basis"] = "not_compared"
		summary["attribution"] = "recorded_claim_only"
		if found["claim"] != nil {
			add("claim", found["claim"], false)
			summary["selected_claim"] = found["claim"]
		}
		if found["use"] != nil {
			add("use", found["use"], false)
			summary["selected_use"] = found["use"]
		}
		d.Parts = append(d.Parts, diagnostics)
		add("record", record, false)
		if record["claims"] != nil {
			add("claims", record["claims"], false)
		}
		raw, _ := base64.StdEncoding.DecodeString(str(doc["raw_base64"]))
		if len(raw) == 0 {
			raw, _ = base64.StdEncoding.DecodeString(str(doc["raw"]))
		}
		body := []byte(str(doc["body"]))
		// Document byte fields use Go's base64 encoding. Body itself is []byte too.
		if decoded, err := base64.StdEncoding.DecodeString(string(body)); err == nil {
			body = decoded
		}
		d.Parts = append(d.Parts, delivery.Binary("carrier", raw), delivery.Text("body", body))
		if data["snapshot_persisted"] == true {
			snapshot, _ := base64.StdEncoding.DecodeString(str(data["snapshot_bytes_base64"]))
			d.Parts = append(d.Parts, delivery.Binary("snapshot", snapshot))
			d.Ref = str(data["exact_ref"])
			d.Lifetime = "persisted_snapshot"
		}
		addReports(&d, body)
		for _, name := range []string{"links", "backlinks", "evidence_uses"} {
			partName := name
			switch name {
			case "diagnostics", "basis", "limits", "result", "request", "summary", "parts":
				partName = "data_" + name
			}
			add(partName, data[name], true)
		}
	} else if q.Operation == "recall" && q.Ref == "" {
		hits, _ := data["hits"].([]any)
		cards := []any{}
		for _, hit := range hits {
			h := object(hit)
			card := selectFields(h, "path", "record_id", "title", "state", "ref")
			text := str(h["text"])
			card["excerpt"] = delivery.Compact(text, 0, 200)
			if ref := str(h["ref"]); ref != "" {
				card["request"] = map[string]any{"format": delivery.Format, "operation": "recall", "ref": ref}
			}
			cards = append(cards, card)
		}
		add("hits", cards, true)
		previewCount := 3
		if q.Limit > 0 {
			previewCount = min(previewCount, q.Limit)
		}
		summary = map[string]any{"total_hits": len(hits), "query": q.Query, "first_hits": cards[:min(previewCount, len(cards))], "scope": "captured memory; excerpts are not full claims"}
		d.Parts = append(d.Parts, diagnostics)
	} else if q.Operation == "check" && q.Action == "observe" {
		obs := object(data["observation"])
		input := object(obs["input"])
		run := object(input["observed"])
		expected := object(input["expected"])
		summary["application_outcome"] = r.Kind
		summary["runner_outcome"] = obs["status"]
		summary["reason_code"] = obs["reason_code"]
		summary["selector"] = run["selector"]
		summary["exit_code"] = run["exit_code"]
		summary["scope"] = expected["scope"]
		summary["observed_basis"] = run["basis"]
		d.Parts = append(d.Parts, diagnostics)
		for _, name := range []string{"observation", "current_basis", "declared_check_binding"} {
			add(name, data[name], false)
		}
		add("expected", expected, false)
		add("selected_events", obs["selected_events"], false)
		add("observation_diagnostics", obs["diagnostics"], false)
		for _, name := range []string{"stdout", "stderr"} {
			raw, _ := base64.StdEncoding.DecodeString(str(run[name+"_base64"]))
			d.Parts = append(d.Parts, delivery.Text(name, raw))
		}
	} else if q.Operation == "check" && q.Action == "prepare" {
		expected := object(data["expected"])
		summary["scope"] = expected["scope"]
		summary["selector"] = expected["selector"]
		summary["claim_ref"] = object(expected["basis"])["claim"]
		for _, name := range []string{"expected", "basis_capture", "command", "run_environment", "toolchain_verification"} {
			add(name, data[name], true)
		}
		d.Parts = append(d.Parts, diagnostics)
	} else if q.Operation == "source" || q.Operation == "fpf" {
		inspection := object(data["inspection"])
		if inspection == nil {
			inspection = data
		}
		unit := object(inspection["unit"])
		if unit != nil {
			summary = selectFields(unit, "kind", "title", "source")
			raw, _ := base64.StdEncoding.DecodeString(str(unit["bytes_base64"]))
			d.Parts = append(d.Parts, delivery.Text("source_body", raw))
			snapshot, _ := base64.StdEncoding.DecodeString(str(data["snapshot_bytes_base64"]))
			if snapshot != nil {
				d.Parts = append(d.Parts, delivery.Binary("source_snapshot", snapshot))
			}
			add("source", unit["source"], false)
		} else {
			summary = data
			add("source_result", data, false)
		}
		d.Parts = append(d.Parts, diagnostics)
	} else {
		names := []string{}
		for name := range data {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names { // Reserve envelope part names; authored keys remain nested data.
			partName := name
			switch name {
			case "diagnostics", "basis", "limits", "result", "request", "summary", "parts":
				partName = "data_" + name
			}
			add(partName, data[name], true)
		}
		if len(summary) == 0 {
			summary = data
		}
		if q.Operation == "change" && q.Action == "preview" {
			add("preview", r.Data, true)
		}
		d.Parts = append(d.Parts, diagnostics)
	}
	add("basis", r.Basis, false)
	add("limits", r.Limits, false)
	add("result", r, false)
	add("request", q, false)
	d.Summary = rawJSON(summary)
	return d
}

// YAML extensions can contain values JSON cannot represent (for example .nan).
// Keep the exact carrier and a labelled YAML authoring projection available;
// never panic, silently drop those fields or call a lossy JSON object complete.
func yamlDelivery(q Request, r Result, cause error) delivery.Document {
	d := delivery.Document{Operation: r.Operation, Kind: r.Kind, IsError: r.Failed(), Basis: r.Basis, Coverage: r.Coverage, Diagnostics: r.Diagnostics, Limits: r.Limits, Lifetime: "transient_cache", PageLimit: q.Limit}
	d.Diagnostics = append(append([]carrier.Diagnostic{}, d.Diagnostics...), carrier.Diagnostic{Code: "json_projection_unavailable", Message: cause.Error() + "; read exact carrier or YAML parts", Severity: "warning"})
	summary := map[string]any{"structured_projection": "YAML only; extensions cannot be expressed as JSON"}
	addYAML := func(name string, v any) {
		raw, err := yaml.Marshal(v)
		if err == nil {
			p := delivery.Text(name, raw)
			p.Label = name + " (YAML representation)"
			d.Parts = append(d.Parts, p)
		}
	}
	if data, ok := r.Data.(map[string]any); ok {
		if found, ok := data["resolution"].(carrier.Resolution); ok && found.Document != nil {
			doc := found.Document
			summary["id"] = doc.Record.ID
			summary["title"] = doc.Record.Title
			summary["exact_ref"] = data["exact_ref"]
			d.Parts = append(d.Parts, delivery.Binary("carrier", doc.Raw), delivery.Text("body", doc.Body))
			addYAML("record", doc.Record)
			if found.Claim != nil {
				addYAML("claim", found.Claim)
			}
			if data["snapshot_persisted"] == true {
				d.Ref = str(data["exact_ref"])
				d.Lifetime = "persisted_snapshot"
				d.Parts = append(d.Parts, delivery.Binary("snapshot", data["snapshot_bytes_base64"].([]byte)))
			}
		}
	}
	addYAML("result", r)
	d.Parts = append(d.Parts, delivery.JSON("diagnostics", d.Diagnostics, false), delivery.JSON("basis", r.Basis, false), delivery.JSON("limits", r.Limits, false))
	d.Summary = rawJSON(summary)
	return d
}

// addReports exposes explicitly delimited JSON as authored data. No prose is
// interpreted as a verdict, and no body bytes or authority are rewritten.
func addReports(d *delivery.Document, body []byte) {
	type report struct {
		Label        string `json:"label"`
		Part         string `json:"part"`
		Start        int    `json:"start_byte"`
		End          int    `json:"end_byte"`
		Digest       string `json:"digest"`
		ContentPart  string `json:"content_part,omitempty"`
		ContentError string `json:"content_error,omitempty"`
	}
	reports := []report{}
	directoryAt := len(d.Parts)
	offset := 0
	label := ""
	start := -1
	kind := ""
	for _, line := range bytes.SplitAfter(body, []byte{'\n'}) {
		text := strings.TrimSpace(string(line))
		if start < 0 {
			if text == "```json" || text == "```jsonl" || text == "```text" {
				kind = strings.TrimPrefix(text, "```")
				start = offset + len(line)
			} else if text != "" {
				label = text
			}
		} else if text == "```" {
			raw := bytes.Clone(body[start:offset])
			name := fmt.Sprintf("report_%d", len(reports)+1)
			p := delivery.Text(name, raw)
			if kind == "json" && json.Valid(raw) {
				p = delivery.Part{Name: name, Media: "json", Raw: raw}
			}
			p.Label = label
			d.Parts = append(d.Parts, p)
			entry := report{Label: label, Part: name, Start: start, End: offset, Digest: carrier.Digest(raw)}
			// Only our explicit retained-data format has a decoded projection.
			// Its digest proves byte identity, never the truth of its contents.
			var attachment struct {
				Format string `json:"format"`
				Digest string `json:"digest"`
				Media  string `json:"media"`
				Raw    []byte `json:"bytes_base64"`
			}
			if kind == "json" && json.Unmarshal(raw, &attachment) == nil && attachment.Format == "haft.retained-part/1" {
				switch {
				case carrier.Digest(attachment.Raw) != attachment.Digest:
					entry.ContentError = "retained_digest_mismatch"
				case attachment.Media == "json" && !json.Valid(attachment.Raw), attachment.Media == "text" && !utf8.Valid(attachment.Raw):
					entry.ContentError = "retained_encoding_invalid"
				case attachment.Media != "json" && attachment.Media != "text" && attachment.Media != "binary":
					entry.ContentError = "retained_media_unknown"
				default:
					entry.ContentPart = name + "_content"
					d.Parts = append(d.Parts, delivery.Part{Name: entry.ContentPart, Label: label + " (verified decoded data)", Media: attachment.Media, Raw: attachment.Raw})
				}
			}
			reports = append(reports, entry)
			start = -1
		}
		offset += len(line)
	}
	if len(reports) > 0 {
		catalog := delivery.JSON("reports", reports, false)
		// Make the directory available before the potentially numerous report parts.
		d.Parts = append(d.Parts[:directoryAt], append([]delivery.Part{catalog}, d.Parts[directoryAt:]...)...)
	}
}
