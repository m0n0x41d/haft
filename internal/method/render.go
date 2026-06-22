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
