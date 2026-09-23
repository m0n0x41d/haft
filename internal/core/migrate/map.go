package migrate

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

var nativeKinds = map[string]string{"DecisionRecord": "decision", "Note": "note", "ProblemCard": "problem", "SolutionPortfolio": "options", "SpecSection": "spec", "Evidence": "evidence", "decision": "decision", "note": "note", "problem": "problem", "options": "options", "spec": "spec", "evidence": "evidence"}
var kindDirs = map[string]string{"decision": "decisions", "note": "notes", "problem": "problems", "options": "options", "spec": "specs", "evidence": "evidence"}
var kindPrefixes = map[string]string{"decision": "dec", "note": "note", "problem": "prob", "options": "opt", "spec": "spec", "evidence": "ev"}
var excludedKinds = map[string]bool{"MethodRun": true, "RefreshReport": true, "WorkCommission": true}

type candidate struct {
	row                          Row
	source, oldID, kind, sidecar string
	fields                       map[string]any
	body                         []byte
	record                       carrier.Record
	item                         Item
	conflict                     bool
}

func rowSource(row Row) string                           { return "sqlite:" + row.Table + ":" + row.Key }
func rowBytes(row Row) []byte                            { b, _ := json.Marshal(row); return append(b, '\n') }
func str(m map[string]any, key string) string            { s, _ := m[key].(string); return s }
func list(m map[string]any, key string) []any            { v, _ := m[key].([]any); return v }
func object(m map[string]any, key string) map[string]any { v, _ := m[key].(map[string]any); return v }
func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func stringList(v any) []string {
	var out []string
	switch a := v.(type) {
	case []string:
		return a
	case []any:
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
func baseFields(row Row) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(row.Columns["structured_data"].Bytes, &m)
	if row.Table == "spec_section_editions" {
		_ = json.Unmarshal(row.Columns["section_json"].Bytes, &m)
	}
	if row.Table == "evidence" || row.Table == "evidence_items" {
		_ = json.Unmarshal(row.Columns["provenance"].Bytes, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	for key, v := range row.Columns {
		if key == "structured_data" || key == "section_json" || key == "provenance" {
			continue
		}
		if utf8.Valid(v.Bytes) && v.StorageClass != "null" {
			if _, exists := m[key]; !exists {
				m[key] = string(v.Bytes)
			}
		}
	}
	return m
}
func nativeID(kind, source string, body []byte, date string) string {
	day := "19700101"
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		day = t.UTC().Format("20060102")
	}
	hash := strings.TrimPrefix(carrier.Digest(append([]byte(source+"\x00"), body...)), "sha256:")
	return kindPrefixes[kind] + "-" + day + "-" + hash[:8]
}
func build(snapshot Snapshot, r Request) Plan {
	p := Plan{Report: Report{Format: "haft.migration-report/1", SourceDigest: snapshot.Digest, DatabasePath: snapshot.DatabasePath, DatabaseFiles: snapshot.DatabaseFiles, CarrierRoot: snapshot.CarrierRoot, OutputRoot: r.OutputRoot, CreatedAt: r.CreatedAt, Scopes: snapshot.Scopes, Excluded: map[string][]string{}, Counts: map[string]int{}, Diagnostics: snapshot.Diagnostics, Limits: []string{"Staging only; no activation, quiesce, installation or source mutation", "Stable observed DB/WAL/SHM copy, interpreted in one private query-only transaction; not an atomic live filesystem capture", "Native historical records do not establish current authority, semantic applicability or evidence truth", "Legacy evidence target editions remain unknown unless recovered independently; converted target hashes are never assigned as historical support", "Unsupported schemas and field/record loss remain visible; no completeness claim for unread scope"}}}
	var candidates []*candidate
	byOld := map[string][]*candidate{}
	sourceIDs := map[string]bool{}
	identityComplete := completeIdentityScope(snapshot)
	excludedIDs := map[string]bool{}
	add := func(row Row, source string, original []byte) {
		fields := baseFields(row)
		oldID := first(text(row, "id"), text(row, "section_id"), str(fields, "id"))
		if oldID != "" {
			sourceIDs[oldID] = true
		}
		oldKind := text(row, "kind")
		if row.Table == "spec_section_editions" {
			oldKind = "SpecSection"
		}
		if row.Table == "evidence_items" || row.Table == "evidence" {
			oldKind = "Evidence"
		}
		if excludedKinds[oldKind] {
			p.Report.Excluded[oldKind] = append(p.Report.Excluded[oldKind], source)
			excludedIDs[oldID] = true
			p.Report.Items = append(p.Report.Items, Item{Source: source, LegacyID: oldID, Kind: oldKind, Disposition: "not_carried_by_policy", Original: source})
			return
		}
		kind := nativeKinds[oldKind]
		if kind == "" {
			p.Report.Items = append(p.Report.Items, Item{Source: source, LegacyID: oldID, Kind: oldKind, Disposition: "unsupported_source", Original: source, Reasons: []string{"Artifact kind is outside the bounded decoder"}, Losses: []Loss{{Kind: "record_loss", Reason: "No native interpretation is declared", AffectedUse: "Recall this original source through its locator"}}})
			return
		}
		suffix := "json"
		if row.Table == "carrier" {
			suffix = "md"
		}
		hash := strings.TrimPrefix(carrier.Digest(original), "sha256:")
		sidecar := "migration/source/" + hash + "." + suffix
		p.Outputs = append(p.Outputs, store.Output{Path: sidecar, Bytes: original})
		c := &candidate{row: row, source: source, oldID: oldID, kind: kind, sidecar: sidecar, fields: fields, body: row.Columns["content"].Bytes, item: Item{Source: source, LegacyID: oldID, Kind: kind, Original: sidecar}}
		for _, key := range []string{"structured_data", "section_json", "provenance"} {
			value := row.Columns[key]
			if value.StorageClass == "null" || len(value.Bytes) == 0 {
				continue
			}
			var decoded map[string]any
			if err := json.Unmarshal(value.Bytes, &decoded); err != nil {
				c.item.Reasons = append(c.item.Reasons, "unsupported_source: "+key+" is not a JSON object; exact source bytes retained")
				continue
			}
			if decoded == nil {
				continue
			}
			_, diagnostics := carrier.ParseYAML(value.Bytes)
			if carrier.HasErrors(diagnostics) {
				c.item.Reasons = append(c.item.Reasons, "source_conflict: "+key+" has duplicate or ambiguous keys; no value is chosen")
			}
			var columnNames []string
			for column := range row.Columns {
				columnNames = append(columnNames, column)
			}
			sort.Strings(columnNames)
			for _, column := range columnNames {
				scalar := row.Columns[column]
				structured, exists := decoded[column]
				if !exists || scalar.StorageClass == "null" || column == "structured_data" || column == "section_json" || column == "provenance" {
					continue
				}
				if s, ok := structured.(string); !ok || s != string(scalar.Bytes) {
					c.item.Reasons = append(c.item.Reasons, "source_conflict: "+key+"."+column+" differs from the scalar source column; both retained")
				}
			}
		}
		if !utf8.Valid(c.body) {
			c.item.Reasons = append(c.item.Reasons, "encoding_error: original bytes preserved; a native UTF-8 carrier is not fabricated")
		}
		for key, value := range row.Columns {
			if value.StorageClass == "text" && !utf8.Valid(value.Bytes) {
				c.item.Reasons = append(c.item.Reasons, "encoding_error in "+key+": exact original bytes preserved")
			}
		}
		candidates = append(candidates, c)
		byOld[oldID] = append(byOld[oldID], c)
	}
	for _, row := range snapshot.Rows {
		if row.Table == "artifacts" || row.Table == "evidence_items" || row.Table == "evidence" || row.Table == "spec_section_editions" {
			add(row, rowSource(row), rowBytes(row))
		}
	}
	paths := make([]string, 0, len(snapshot.Carriers))
	for file := range snapshot.Carriers {
		paths = append(paths, file)
	}
	sort.Strings(paths)
	for _, file := range paths {
		raw := snapshot.Carriers[file]
		source := "carrier:" + file
		front, body, err := carrier.SplitFrontmatter(raw)
		if err != nil {
			identityComplete = false
			p.Report.Items = append(p.Report.Items, preserveUnsupported(&p, source, raw, "No recognized carrier frontmatter; content needs an explicit rewrite"))
			continue
		}
		node, ds := carrier.ParseYAML(front)
		if carrier.HasErrors(ds) {
			identityComplete = false
			p.Report.Items = append(p.Report.Items, preserveUnsupported(&p, source, raw, "Malformed or ambiguous source frontmatter"))
			continue
		}
		m := map[string]any{}
		if err := node.Decode(&m); err != nil {
			identityComplete = false
			p.Report.Items = append(p.Report.Items, preserveUnsupported(&p, source, raw, err.Error()))
			continue
		}
		if str(m, "format") == "haft.terms/1" {
			terms := carrier.ParseTerms(raw)
			if !carrier.HasErrors(terms.Diagnostics) {
				p.Outputs = append(p.Outputs, store.Output{Path: "specs/terms.md", Bytes: raw})
				p.Report.Items = append(p.Report.Items, Item{Source: source, Kind: "terms", Disposition: "preserved", Destination: "specs/terms.md", Original: source})
				continue
			}
		}
		fields, _ := json.Marshal(m)
		// YAML timestamps may decode as time.Time. JSON normalization retains the
		// same timestamp string for both the structured and scalar row positions.
		normalized := map[string]any{}
		_ = json.Unmarshal(fields, &normalized)
		row := Row{Table: "carrier", Key: file, Columns: map[string]Value{"content": {StorageClass: "text", Bytes: body}, "structured_data": {StorageClass: "text", Bytes: fields}}}
		for _, key := range []string{"id", "kind", "title", "status", "created_at", "updated_at"} {
			if _, exists := normalized[key]; exists {
				row.Columns[key] = Value{StorageClass: "text", Bytes: []byte(str(normalized, key))}
			}
		}
		add(row, source, raw)
	}
	// DB and carrier remain separate source positions. Divergence never silently
	// elects one as current; both raw carriers remain addressable for assistance.
	for _, cs := range byOld {
		for i, a := range cs {
			for _, b := range cs[i+1:] {
				if a.row.Table == b.row.Table || a.row.Table != "carrier" && b.row.Table != "carrier" {
					continue
				}
				if string(a.body) != string(b.body) || sharedFieldsConflict(a.fields, b.fields) {
					a.conflict = true
					b.conflict = true
				}
			}
		}
	}
	for _, c := range candidates {
		mapCandidate(c)
		needsTerms := len(c.record.Terms) > 0
		for _, claim := range c.record.Claims {
			needsTerms = needsTerms || len(claim.Terms) > 0
		}
		if needsTerms {
			found := false
			for _, output := range p.Outputs {
				if output.Path == "specs/terms.md" {
					found = true
					terms := carrier.ParseTerms(output.Bytes)
					ds := append(terms.Diagnostics, carrier.ValidateTermRefs(c.record, terms.Terms)...)
					for _, diagnostic := range ds {
						if diagnostic.Severity == "error" {
							c.item.Reasons = append(c.item.Reasons, diagnostic.Path+": "+diagnostic.Message)
						}
					}
				}
			}
			if !found {
				c.item.Reasons = append(c.item.Reasons, "Declared term interpretation is unavailable; original terms retained without invented definitions")
			}
		}
	}
	validByOld := map[string]*candidate{}
	for _, c := range candidates {
		if c.conflict {
			c.item.Reasons = append(c.item.Reasons, "source_conflict: DB and carrier disagree; neither is chosen silently")
		}
		if len(c.item.Reasons) == 0 && !carrier.HasErrors(carrier.Validate(c.record, false)) {
			if previous := validByOld[c.oldID]; previous == nil || previous.row.Table == "carrier" || c.row.Table == "artifacts" {
				validByOld[c.oldID] = c
			}
		}
	}
	// Only an artifact-table identity or one unambiguous source identity can be
	// used for a native relation. A shared ID across evidence tables/section
	// editions does not silently elect one historical position.
	for oldID, cs := range byOld {
		count := 0
		for _, c := range cs {
			if c.row.Table != "carrier" && len(c.item.Reasons) == 0 && !carrier.HasErrors(carrier.Validate(c.record, false)) {
				count++
			}
		}
		if current := validByOld[oldID]; count > 1 && current != nil && current.row.Table != "artifacts" {
			delete(validByOld, oldID)
		}
	}
	for _, row := range snapshot.Rows {
		if row.Table == "artifacts" || row.Table == "evidence_items" || row.Table == "evidence" || row.Table == "spec_section_editions" {
			continue
		}
		sourceID := first(text(row, "artifact_id"), text(row, "source_id"))
		if excludedIDs[sourceID] {
			p.Report.Excluded["associations_from_excluded"] = append(p.Report.Excluded["associations_from_excluded"], rowSource(row))
			continue
		}
		raw := rowBytes(row)
		sidecar := "migration/source/" + strings.TrimPrefix(carrier.Digest(raw), "sha256:") + ".json"
		p.Outputs = append(p.Outputs, store.Output{Path: sidecar, Bytes: raw})
		item := Item{Source: rowSource(row), LegacyID: sourceID, Kind: row.Table, Original: sidecar, Disposition: "weakened"}
		source := validByOld[sourceID]
		if source != nil {
			related := source.record.Legacy["related_source_rows"]
			refs, _ := related.([]string)
			source.record.Legacy["related_source_rows"] = append(refs, sidecar)
		}
		switch row.Table {
		case "artifact_links":
			targetID, oldType := text(row, "target_id"), text(row, "link_type")
			target := validByOld[targetID]
			if source != nil && target != nil {
				source.record.Links = append(source.record.Links, carrier.Link{Kind: "relates_to", Target: target.record.ID, LegacyType: oldType, Extra: carrier.Extra{"legacy_source": rowSource(row)}})
				item.Losses = append(item.Losses, Loss{Kind: "weakened", Field: "link_type", Original: oldType, Reason: "Historical relation retained as navigation; premise, succession and authority are not inferred", AffectedUse: "Context/impact must read the original relation"})
			} else {
				item.Disposition = "resolution_unknown"
				item.Reasons = append(item.Reasons, "Endpoint has no native mapped carrier; original endpoints retained, no guessed replacement")
				item.Losses = append(item.Losses, Loss{Kind: "field_loss", Field: "native_link", Reason: "Original endpoint is unavailable or requires rewrite", AffectedUse: "Native relation traversal"})
				if sourceIDs[targetID] && sourceIDs[sourceID] {
					item.Disposition = "new_unresolved"
					item.Reasons = append(item.Reasons, "Both endpoints exist in the source; native conversion is incomplete or ambiguous")
				} else if identityComplete {
					item.Disposition = "existing_unresolved"
					item.Reasons = append(item.Reasons, "An endpoint is absent from the fully read bounded source identity tables and carriers")
				}
			}
		case "affected_files", "affected_symbols", "artifact_symbol_bindings":
			selector := "file:" + text(row, "file_path")
			if row.Table != "affected_files" {
				selector = "sym:" + text(row, "file_path") + "::" + text(row, "symbol_name")
			}
			if source != nil && carrier.ValidateSelector(selector) == nil {
				source.record.Links = append(source.record.Links, carrier.Link{Kind: "relates_to", Target: selector, LegacyType: row.Table, Extra: carrier.Extra{"legacy_role": "unclassified_affected_scope", "legacy_source": rowSource(row)}})
			}
			item.Losses = append(item.Losses, Loss{Kind: "weakened", Field: "role", Original: row.Table, Reason: "Affected location does not establish a normative constraint or current symbol identity", AffectedUse: "Exact governance/implementation inference"})
		case "spec_section_baselines":
			item.Reasons = append(item.Reasons, "Historical baseline retained as source material; approval and validation are not imported")
		}
		p.Report.Items = append(p.Report.Items, item)
	}
	for _, c := range candidates {
		ds := carrier.Validate(c.record, false)
		for _, d := range ds {
			if d.Severity == "error" {
				c.item.Reasons = append(c.item.Reasons, d.Path+": "+d.Message)
			}
		}
		if len(c.item.Reasons) > 0 {
			c.item.Disposition = "needs_rewrite"
			c.item.Losses = append(c.item.Losses, Loss{Kind: "record_loss", Reason: "Native carrier was not manufactured from missing or conflicting semantics", AffectedUse: "Native governing/evidence use; original remains readable"})
			queueID := "queue-" + strings.TrimPrefix(carrier.Digest([]byte(c.source+"\x00"+c.sidecar)), "sha256:")[:16]
			p.Report.Queue = append(p.Report.Queue, QueueItem{ID: queueID, Source: c.source, Original: c.sidecar, Kind: c.kind, Reason: strings.Join(c.item.Reasons, "; "), Priority: "select_for_current_work", Status: "pending"})
		} else {
			chosen := validByOld[c.oldID]
			if chosen != nil && chosen != c && c.row.Table == "carrier" {
				c.record.ID = chosen.record.ID
			}
			body := c.body
			raw, err := carrier.Encode(c.record, body)
			if err != nil {
				c.item.Disposition = "needs_rewrite"
				c.item.Reasons = append(c.item.Reasons, err.Error())
			} else {
				c.item.Disposition = "weakened"
				c.item.NativeID = c.record.ID
				c.item.Destination = kindDirs[c.kind] + "/" + c.record.ID + ".md"
				if chosen == nil || chosen == c || c.row.Table != "carrier" {
					p.Outputs = append(p.Outputs, store.Output{Path: c.item.Destination, Bytes: raw})
				}
				c.item.Losses = append(c.item.Losses, Loss{Kind: "weakened", Field: "status/authority", Original: c.record.LegacyStatus, Reason: "Historical migrated record remains non-governing; no operator confirmation added", AffectedUse: "Use as a current accepted norm"})
			}
		}
		p.Report.Items = append(p.Report.Items, c.item)
		p.Report.Aliases = append(p.Report.Aliases, Alias{LegacyID: c.oldID, Source: c.source, NativeID: c.item.NativeID, Original: c.sidecar, Disposition: c.item.Disposition})
	}
	for key := range p.Report.Excluded {
		sort.Strings(p.Report.Excluded[key])
	}
	sort.Slice(p.Report.Items, func(i, j int) bool { return p.Report.Items[i].Source < p.Report.Items[j].Source })
	sort.Slice(p.Report.Queue, func(i, j int) bool { return p.Report.Queue[i].ID < p.Report.Queue[j].ID })
	sort.Slice(p.Report.Aliases, func(i, j int) bool { return p.Report.Aliases[i].Source < p.Report.Aliases[j].Source })
	for _, item := range p.Report.Items {
		p.Report.Counts[item.Disposition]++
		for _, loss := range item.Losses {
			p.Report.Counts["loss:"+loss.Kind]++
		}
	}
	// Identical source sidecars may be shared; conflicting destination bytes are
	// left as duplicate outputs so the normal Store rejects rather than overwrites.
	seen := map[string]string{}
	outputs := []store.Output{}
	for _, o := range p.Outputs {
		digest := carrier.Digest(o.Bytes)
		if seen[o.Path] == digest {
			continue
		}
		seen[o.Path] = digest
		outputs = append(outputs, o)
	}
	p.Outputs = outputs
	return p
}
func preserveUnsupported(p *Plan, source string, raw []byte, reason string) Item {
	hash := strings.TrimPrefix(carrier.Digest(raw), "sha256:")
	sidecar := "migration/source/" + hash + ".md"
	p.Outputs = append(p.Outputs, store.Output{Path: sidecar, Bytes: raw})
	queueID := "queue-" + strings.TrimPrefix(carrier.Digest([]byte(source+"\x00"+sidecar)), "sha256:")[:16]
	p.Report.Queue = append(p.Report.Queue, QueueItem{ID: queueID, Source: source, Original: sidecar, Kind: "unknown", Reason: reason, Priority: "select_for_current_work", Status: "pending"})
	return Item{Source: source, Disposition: "needs_rewrite", Original: sidecar, Reasons: []string{reason}, Losses: []Loss{{Kind: "record_loss", Reason: reason, AffectedUse: "Native structured recall"}}}
}

func mapCandidate(c *candidate) {
	f := c.fields
	created := str(f, "created_at")
	oldStatus := str(f, "status")
	status := "proposed"
	if oldStatus != "draft" && oldStatus != "proposed" {
		status = "active"
	}
	r := carrier.Record{Format: "haft/1", ID: nativeID(c.kind, c.source, rowBytes(c.row), created), Kind: c.kind, Title: str(f, "title"), Status: status, Origin: "migrated_9x", About: first(str(f, "about"), str(f, "decision_subject_ref")), CreatedAt: created, LegacyStatus: oldStatus, Legacy: carrier.Extra{"id": c.oldID, "source_locator": c.source, "source_bytes": c.sidecar, "source_fields": f, "historical_edition": "unknown"}}
	r.Object = str(f, "object")
	r.Question = str(f, "question")
	r.ReceivingUse = str(f, "receiving_use")
	switch c.kind {
	case "decision":
		r.Disposition = first(str(f, "disposition"), str(object(f, "choice_result"), "next_move"))
		r.Chosen = first(str(f, "chosen"), str(f, "selected_title"))
		r.Why = first(str(f, "why"), str(f, "why_selected"))
		r.ChoiceRule = first(str(f, "choice_rule"), str(f, "selection_policy"))
		r.NoAlternativeReason = str(f, "no_alternative_reason")
		r.Probe = str(f, "probe")
		r.RerouteTo = str(f, "reroute_to")
		if r.Disposition == "" && r.Chosen != "" {
			r.Disposition = "choose_now"
		}
		decodeField(f, "options", &r.Options)
		if len(r.Options) == 0 {
			for _, value := range list(f, "why_not_others") {
				m, _ := value.(map[string]any)
				title := first(str(m, "variant"), str(m, "title"))
				reason := str(m, "reason")
				if title != "" {
					id := "legacy-" + strings.TrimPrefix(carrier.Digest([]byte(title)), "sha256:")[:8]
					r.Options = append(r.Options, carrier.Option{ID: id, Summary: title, Verdict: "rejected", Reason: reason})
				}
			}
		}
		for _, value := range list(f, "governance_targets") {
			m, _ := value.(map[string]any)
			ref := str(m, "ref")
			if carrier.ValidateSelector(ref) == nil && r.Disposition == "choose_now" {
				r.Constrains = append(r.Constrains, ref)
			}
		}
	case "problem":
		r.Signal = str(f, "signal")
		r.Acceptance = str(f, "acceptance")
	case "options":
		r.NextUse = str(f, "next_use")
		r.Probe = str(f, "probe")
		decodeField(f, "options", &r.Options)
		decodeField(f, "comparison", &r.Comparison)
	case "spec":
		r.Slug = str(f, "slug")
		r.Terms = stringList(f["terms"])
		if refs, ok := f["target_refs"]; ok {
			c.item.Losses = append(c.item.Losses, Loss{Kind: "field_loss", Field: "target_refs", Original: refs, Reason: "Historical targets are preserved as source material; they do not prove current subject or constraint roles", AffectedUse: "Native target traversal"})
		}
		for _, value := range list(f, "claims") {
			m, _ := value.(map[string]any)
			kind := str(m, "kind")
			if carrier.Quadrant(kind) == "" {
				c.item.Losses = append(c.item.Losses, Loss{Kind: "field_loss", Field: "claims." + str(m, "id"), Original: m, Reason: "Legacy quadrant labels do not determine a new claim kind; preserved in historical source", AffectedUse: "Native claim/check relation"})
				continue
			}
			claim := carrier.Claim{ID: str(m, "id"), Kind: kind, Text: first(str(m, "text"), str(m, "statement")), Extra: carrier.Extra{"legacy_claim": m}}
			claim.Terms = stringList(m["terms"])
			decodeField(m, "checks", &claim.Checks)
			decodeField(m, "implemented_by", &claim.ImplementedBy)
			decodeField(m, "examples", &claim.Examples)
			for _, key := range []string{"refs", "support_refs", "governing_pattern_refs", "evidence_refs", "evidence_inputs", "scope"} {
				if value, ok := m[key]; ok {
					c.item.Losses = append(c.item.Losses, Loss{Kind: "field_loss", Field: "claims." + claim.ID + "." + key, Original: value, Reason: "Historical relation/scope is retained in legacy_claim; exact native address, direction and interpretation are not established", AffectedUse: "Native claim relation or scoped reliance"})
				}
			}
			if (kind == "law" || kind == "guard") && len(claim.Checks) == 0 {
				claim.Unchecked = "No validation harness was reconstructed from the historical source"
			}
			r.Claims = append(r.Claims, claim)
		}
	case "evidence":
		r.Claim = first(str(f, "claim"), string(c.body))
		r.ObservedAt = first(str(f, "observed_at"), str(f, "created_at"))
		r.Method = str(f, "method")
		r.Source = first(str(f, "source"), str(f, "carrier_ref"))
		decodeField(f, "basis", &r.Basis)
		// Even a complete observation does not prove which historical claim
		// edition its verdict addressed. Do not create uses on converted hashes.
		r.Legacy["target_ref"] = first(str(f, "artifact_ref"), str(f, "holon_id"))
		r.Legacy["claim_scope"] = f["claim_scope"]
		r.Legacy["claim_refs"] = f["claim_refs"]
		r.Legacy["verdict"] = f["verdict"]
		c.item.Losses = append(c.item.Losses, Loss{Kind: "field_loss", Field: "uses", Original: map[string]any{"target": r.Legacy["target_ref"], "scope": f["claim_scope"], "claim_refs": f["claim_refs"], "verdict": f["verdict"]}, Reason: "Historical target edition is unknown; current converted hash cannot stand in for it", AffectedUse: "Support for an exact native claim"})
	}
	for _, file := range stringList(object(f, "implementation_footprint")["files"]) {
		selector := "file:" + file
		if carrier.ValidateSelector(selector) == nil {
			r.Links = append(r.Links, carrier.Link{Kind: "relates_to", Target: selector, LegacyType: "implementation_footprint", Extra: carrier.Extra{"legacy_role": "implementation_footprint"}})
		} else {
			c.item.Losses = append(c.item.Losses, Loss{Kind: "field_loss", Field: "implementation_footprint.files", Original: file, Reason: "Legacy path is not a valid native selector; original remains in source_fields", AffectedUse: "Native file navigation"})
		}
	}
	if !utf8.Valid(c.body) {
		r.Legacy["encoding_error"] = true
	}
	c.record = r
}

func completeIdentityScope(s Snapshot) bool {
	if s.DatabasePath != "" {
		for _, table := range []string{"artifacts", "evidence", "evidence_items", "spec_section_editions"} {
			complete := false
			for _, scope := range s.Scopes {
				if scope.Source == "sqlite:"+table && (scope.Disposition == "read" || scope.Disposition == "absent") {
					complete = true
				}
			}
			if !complete {
				return false
			}
		}
	}
	if s.CarrierRoot != "" {
		for _, scope := range s.Scopes {
			if strings.HasPrefix(scope.Source, "carriers:") && scope.Disposition == "read" {
				return true
			}
		}
		return false
	}
	return s.DatabasePath != ""
}
func decodeField(m map[string]any, key string, dst any) {
	if value, ok := m[key]; ok {
		b, err := json.Marshal(value)
		if err == nil {
			_ = json.Unmarshal(b, dst)
		}
	}
}
func reportPath(digest string) string {
	return path.Join("migration/reports", strings.TrimPrefix(digest, "sha256:")+".json")
}
func sortQueue(items []QueueItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func sharedFieldsConflict(a, b map[string]any) bool {
	keys := map[string]bool{}
	for key := range a {
		keys[key] = true
	}
	for key := range b {
		keys[key] = true
	}
	for key := range keys {
		av, leftExists := a[key]
		bv, exists := b[key]
		if key == "content" || key == "structured_data" {
			continue
		}
		if leftExists != exists {
			return true
		}
		left, e1 := json.Marshal(av)
		right, e2 := json.Marshal(bv)
		if e1 == nil && e2 == nil && string(left) != string(right) {
			return true
		}
	}
	return false
}
