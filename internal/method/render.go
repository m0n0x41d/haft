package method

import (
	"fmt"
	"strings"
)

func RenderRunBody(run MethodRun) string {
	var b strings.Builder
	b.WriteString("# Method Run\n\n")
	b.WriteString(fmt.Sprintf("- Status: %s\n", run.Status))
	b.WriteString(fmt.Sprintf("- Catalog: %s@%s\n", run.CatalogID, run.CatalogVersion))
	b.WriteString(fmt.Sprintf("- Task kind: %s\n", run.TaskSignature.NormalizedTaskKind))
	b.WriteString(fmt.Sprintf("- Ceremony: %s — %s\n", run.TaskSignature.Ceremony, run.TaskSignature.CeremonyReason))
	if run.TaskSignature.Task != "" {
		b.WriteString(fmt.Sprintf("- Task: %s\n", run.TaskSignature.Task))
	}
	b.WriteString("\n")

	if len(run.Methods) == 0 {
		b.WriteString("## Methods\n\nNo method gates applied.\n")
	} else {
		b.WriteString("## Methods\n\n")
		for _, card := range run.Methods {
			b.WriteString(fmt.Sprintf("### %s `%s`\n\n", card.Title, card.ID))
			if posture := RenderSourcePosture(card.SourcePosture); posture != "" {
				b.WriteString(posture)
				b.WriteString("\n\n")
			}
			if len(card.SourcePatternRefs) > 0 {
				b.WriteString(fmt.Sprintf("Source pattern refs: %s\n\n", strings.Join(card.SourcePatternRefs, ", ")))
			}
			if card.WhyApplies != "" {
				b.WriteString(fmt.Sprintf("Why applies: %s\n\n", card.WhyApplies))
			}
			if card.Intent != "" {
				b.WriteString(fmt.Sprintf("Intent: %s\n\n", card.Intent))
			}
			if len(card.HardGates) > 0 {
				b.WriteString("Hard gates:\n")
				for _, gate := range card.HardGates {
					b.WriteString(fmt.Sprintf("- `%s` (%s/%s): %s\n",
						gate.ID,
						gate.Kind,
						gate.CheckLevel,
						gate.PassCondition,
					))
				}
				b.WriteString("\n")
			}
		}
	}

	if len(run.CarryThrough) > 0 {
		b.WriteString("## Carry-through\n\n")
		for _, item := range run.CarryThrough {
			item = NormalizeCarryThroughItem(item)
			b.WriteString(fmt.Sprintf("- `%s#%s`: %s accepted by %s (%s/%s)\n",
				item.SourceRef,
				item.SourceItemRef,
				item.Disposition,
				item.AcceptanceRef,
				item.AcceptanceRefKind,
				item.AcceptanceRefStatus,
			))
		}
		b.WriteString("\n")
	}

	if len(run.Checkpoints) > 0 {
		b.WriteString("## Checkpoints\n\n")
		for _, record := range run.Checkpoints {
			switch record.RecordKind {
			case CheckpointRecordOpen:
				b.WriteString(fmt.Sprintf("- open `%s`: target=%s check=%s expires=%s\n",
					record.CheckpointID,
					record.TargetRef,
					record.CheckRef,
					record.ExpiresAt,
				))
			case CheckpointRecordClose:
				b.WriteString(fmt.Sprintf("- close `%s`: outcome=%s closed=%s\n",
					record.CheckpointID,
					record.Outcome,
					record.ClosedAt,
				))
			}
		}
		b.WriteString("\n")
	}

	if run.Closeout != nil {
		b.WriteString("## Closeout\n\n")
		if len(run.Closeout.ChangedFiles) > 0 {
			b.WriteString("Changed files:\n")
			for _, file := range run.Closeout.ChangedFiles {
				b.WriteString(fmt.Sprintf("- %s\n", file))
			}
			b.WriteString("\n")
		}
		if len(run.Closeout.GateResults) > 0 {
			b.WriteString("Gate results:\n")
			for _, result := range run.Closeout.GateResults {
				b.WriteString(fmt.Sprintf("- `%s`: %s\n", result.GateID, result.Status))
			}
			b.WriteString("\n")
		}
		if run.Closeout.Verification.Result != "" {
			b.WriteString(fmt.Sprintf("Verification: %s\n\n", run.Closeout.Verification.Result))
		}
		if len(run.Closeout.Waivers) > 0 {
			b.WriteString("Waivers:\n")
			for _, waiver := range run.Closeout.Waivers {
				b.WriteString(fmt.Sprintf("- `%s`: %s\n", waiver.GateID, waiver.Reason))
			}
			b.WriteString("\n")
		}
		if len(run.Closeout.CarryThrough) > 0 {
			b.WriteString("Carry-through dispositions:\n")
			for _, item := range run.Closeout.CarryThrough {
				item = NormalizeCarryThroughItem(item)
				b.WriteString(fmt.Sprintf("- `%s#%s`: %s (%s/%s)\n",
					item.SourceRef,
					item.SourceItemRef,
					item.Disposition,
					item.AcceptanceRefKind,
					item.AcceptanceRefStatus,
				))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func RenderSourcePosture(posture SourcePosture) string {
	parts := make([]string, 0, 4)
	if posture.SourceKind != "" {
		parts = append(parts, "source_kind="+posture.SourceKind)
	}
	if posture.SourceEdition != "" {
		parts = append(parts, "source_edition="+posture.SourceEdition)
	}
	if posture.Normativity != "" {
		parts = append(parts, "normativity="+posture.Normativity)
	}
	if posture.AuthorityBoundary != "" {
		parts = append(parts, "authority_boundary="+posture.AuthorityBoundary)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Source posture: " + strings.Join(parts, " · ")
}
