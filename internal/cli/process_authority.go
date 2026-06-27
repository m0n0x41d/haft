package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

const (
	processAuthorityKind      = "haft_process_authority"
	processAuthorityAuthority = "read_only_process_authority_index_not_source_of_truth_not_processpattern"
)

const maxProcessAuthorityTextEntries = 20

var processAuthorityJSON bool

var processAuthorityCmd = &cobra.Command{
	Use:   "authority",
	Short: "Read-only derived process authority index",
	Long: `Build a read-only ProcessAuthorityEntry index from current MethodPack
definitions and kernel interface contracts.

The index is a discovery surface only. It is not a ProcessPattern subsystem,
not an approval, not evidence truth, and not an enforcement authority.`,
	RunE: runProcessAuthority,
}

type processAuthorityReport struct {
	Kind              string                  `json:"kind"`
	SchemaVersion     int                     `json:"schema_version"`
	Authority         string                  `json:"authority"`
	AuthorityBoundary string                  `json:"authority_boundary"`
	Sources           []string                `json:"sources"`
	Summary           processAuthoritySummary `json:"summary"`
	Entries           []ProcessAuthorityEntry `json:"entries"`
	Notes             []string                `json:"notes,omitempty"`
}

type processAuthoritySummary struct {
	Total       int            `json:"total"`
	ByClaimKind map[string]int `json:"by_claim_kind"`
	ByLifecycle map[string]int `json:"by_lifecycle"`
}

type ProcessAuthorityEntry struct {
	AuthorityKey    string   `json:"authority_key"`
	ClaimKind       string   `json:"claim_kind"`
	BoundedContext  string   `json:"bounded_context"`
	TargetRef       string   `json:"target_ref"`
	SourceRef       string   `json:"source_ref"`
	SourceLocator   string   `json:"source_locator"`
	SourceDigest    string   `json:"source_digest"`
	LifecycleStatus string   `json:"lifecycle_status"`
	SuccessorRefs   []string `json:"successor_refs,omitempty"`
	CarrierRefs     []string `json:"carrier_refs,omitempty"`
	ValidUntil      string   `json:"valid_until,omitempty"`
}

func init() {
	processAuthorityCmd.Flags().BoolVar(&processAuthorityJSON, "json", false, "print structured JSON output")
	processCmd.AddCommand(processAuthorityCmd)
}

func runProcessAuthority(cmd *cobra.Command, args []string) error {
	report, err := buildProcessAuthorityReport()
	if err != nil {
		return err
	}
	if processAuthorityJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}
	return writeProcessAuthorityText(cmd.OutOrStdout(), report)
}

func buildProcessAuthorityReport() (processAuthorityReport, error) {
	catalog := methodpkg.BuiltinCatalog()
	if err := methodpkg.ValidateCatalog(catalog); err != nil {
		return processAuthorityReport{}, err
	}

	interfaceCatalog := haftInterfaceCatalog()
	interfaceDigest := buildInterfaceContractGenerationReport(interfaceCatalog).SourceDigest

	entries := make([]ProcessAuthorityEntry, 0, len(catalog.Methods)*3+len(interfaceCatalog))
	entries = append(entries, methodAuthorityEntries(catalog)...)
	entries = append(entries, interfaceAuthorityEntries(interfaceCatalog, interfaceDigest)...)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].AuthorityKey < entries[j].AuthorityKey
	})

	return processAuthorityReport{
		Kind:              processAuthorityKind,
		SchemaVersion:     1,
		Authority:         processAuthorityAuthority,
		AuthorityBoundary: "derived_index_only_not_processpattern_not_approval_not_evidence_truth_not_gate_passage_not_enforcement",
		Sources: []string{
			"methodpack:" + catalog.ID + "@" + catalog.Version,
			"kernel_interface_catalog@" + interfaceDigest,
		},
		Summary: summarizeProcessAuthorityEntries(entries),
		Entries: entries,
		Notes: []string{
			"Canonical authority remains in MethodPack definitions and kernel interface contracts.",
			"Entries are discovery records; they do not create approval, evidence truth, gate passage, or a ProcessPattern subsystem.",
		},
	}, nil
}

func methodAuthorityEntries(catalog methodpkg.Catalog) []ProcessAuthorityEntry {
	var entries []ProcessAuthorityEntry
	for _, definition := range catalog.Methods {
		methodRef := "method:" + definition.ID
		sourceRef := catalog.ID + "@" + catalog.Version + ":" + definition.ID
		sourceDigest := interfaceContractGenerationDigest(definition)
		entries = append(entries, ProcessAuthorityEntry{
			AuthorityKey:    "method:" + catalog.ID + ":" + definition.ID + ":step",
			ClaimKind:       "method_step",
			BoundedContext:  "methodpack:" + catalog.ID,
			TargetRef:       methodRef,
			SourceRef:       sourceRef,
			SourceLocator:   "internal/method/builtin.go#" + definition.ID,
			SourceDigest:    sourceDigest,
			LifecycleStatus: definition.Lifecycle.Status,
			SuccessorRefs:   append([]string(nil), definition.Lifecycle.SuccessorRefs...),
			CarrierRefs:     append([]string(nil), definition.CarrierRefs...),
			ValidUntil:      definition.Lifecycle.ValidUntil,
		})
		entries = append(entries, ProcessAuthorityEntry{
			AuthorityKey:    "method:" + catalog.ID + ":" + definition.ID + ":authority_boundary",
			ClaimKind:       "authority_boundary",
			BoundedContext:  "methodpack:" + catalog.ID,
			TargetRef:       methodRef + "#source_posture",
			SourceRef:       sourceRef,
			SourceLocator:   "internal/method/catalog.go#SourcePosture",
			SourceDigest:    sourceDigest,
			LifecycleStatus: definition.Lifecycle.Status,
			CarrierRefs:     append([]string(nil), definition.CarrierRefs...),
			ValidUntil:      definition.Lifecycle.ValidUntil,
		})
		for _, gate := range definition.HardGates {
			entries = append(entries, ProcessAuthorityEntry{
				AuthorityKey:    "method:" + catalog.ID + ":" + definition.ID + ":gate:" + gate.ID,
				ClaimKind:       "hard_gate",
				BoundedContext:  "methodpack:" + catalog.ID,
				TargetRef:       methodRef + "#gate:" + gate.ID,
				SourceRef:       sourceRef,
				SourceLocator:   "internal/method/builtin.go#" + definition.ID + ".hard_gates." + gate.ID,
				SourceDigest:    sourceDigest,
				LifecycleStatus: definition.Lifecycle.Status,
				SuccessorRefs:   append([]string(nil), definition.Lifecycle.SuccessorRefs...),
				CarrierRefs:     append([]string(nil), definition.CarrierRefs...),
				ValidUntil:      definition.Lifecycle.ValidUntil,
			})
		}
	}
	return entries
}

func interfaceAuthorityEntries(catalog []interfaceCapability, sourceDigest string) []ProcessAuthorityEntry {
	entries := make([]ProcessAuthorityEntry, 0, len(catalog))
	for _, capability := range catalog {
		entries = append(entries, ProcessAuthorityEntry{
			AuthorityKey:    "interface:" + capability.ID,
			ClaimKind:       "interface_contract",
			BoundedContext:  "kernel_interface_catalog",
			TargetRef:       "interface:" + capability.ID,
			SourceRef:       "kernel_interface_catalog",
			SourceLocator:   "internal/cli/interface.go#" + capability.ID,
			SourceDigest:    sourceDigest,
			LifecycleStatus: "current",
			CarrierRefs:     interfaceCapabilityCarrierRefs(capability),
		})
	}
	return entries
}

func interfaceCapabilityCarrierRefs(capability interfaceCapability) []string {
	refs := []string{"internal/cli/interface.go"}
	if capability.CurrentExecution.DiscoveryCommand != "" {
		refs = append(refs, capability.CurrentExecution.DiscoveryCommand)
	}
	if capability.CurrentExecution.CLICommand != "" {
		refs = append(refs, capability.CurrentExecution.CLICommand)
	}
	return refs
}

func summarizeProcessAuthorityEntries(entries []ProcessAuthorityEntry) processAuthoritySummary {
	summary := processAuthoritySummary{
		Total:       len(entries),
		ByClaimKind: map[string]int{},
		ByLifecycle: map[string]int{},
	}
	for _, entry := range entries {
		summary.ByClaimKind[entry.ClaimKind]++
		summary.ByLifecycle[entry.LifecycleStatus]++
	}
	return summary
}

func writeProcessAuthorityText(w io.Writer, report processAuthorityReport) error {
	if _, err := fmt.Fprintf(w, "Haft process authority index\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "entries=%d authority=%s\n", report.Summary.Total, report.Authority); err != nil {
		return err
	}
	entries := report.Entries
	if len(entries) > maxProcessAuthorityTextEntries {
		entries = entries[:maxProcessAuthorityTextEntries]
	}
	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "- %s %s -> %s (%s)\n", entry.ClaimKind, entry.AuthorityKey, entry.TargetRef, entry.LifecycleStatus); err != nil {
			return err
		}
	}
	if omitted := len(report.Entries) - len(entries); omitted > 0 {
		if _, err := fmt.Fprintf(w, "... and %d more; run `haft process authority --json` for full entries\n", omitted); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "sources=%s\n", strings.Join(report.Sources, ", ")); err != nil {
		return err
	}
	return nil
}
