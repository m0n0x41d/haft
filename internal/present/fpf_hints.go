package present

import "strings"

// FPFPhaseHint returns a compact nudge for the given reasoning phase,
// reminding the agent which FPF patterns are relevant and how to retrieve them.
// Injected into tool responses for reasoning actions (frame, characterize,
// explore, compare, decide, verify). Returns empty string for non-reasoning actions.
func FPFPhaseHint(phase string) string {
	hint, ok := fpfPhaseHints[strings.ToLower(strings.TrimSpace(phase))]
	if !ok {
		return ""
	}
	return hint
}

var fpfPhaseHints = map[string]string{
	"frame": `
── FPF patterns for framing ──
FRAME-01 (signal typing), FRAME-02 (scope boundary), FRAME-03 (acceptance criteria), FRAME-05 (problem typing), CHR-01 (indicator roles)
Retrieve: haft_query(action="fpf", query="FRAME-01") | Full phase: haft_query(action="fpf", query="frame problem signal scope")
`,
	"characterize": `
── FPF patterns for characterization ──
CHR-01 (indicator roles: constraint/target/observation), CHR-02 (characterization protocol), CHR-04 (assurance tuple F-G-R-CL), CHR-09 (parity plan)
Retrieve: haft_query(action="fpf", query="CHR-01") | Full phase: haft_query(action="fpf", query="characterize indicator parity")
`,
	"explore": `
── FPF patterns for exploration ──
EXP-01 (abductive loop), EXP-02 (rival preservation), EXP-04 (WLNK per variant), EXP-05 (stepping stones), EXP-07 (portfolio thinking)
Retrieve: haft_query(action="fpf", query="EXP-01") | Full phase: haft_query(action="fpf", query="explore variant abduction WLNK")
`,
	"compare": `
── FPF patterns for comparison ──
CMP-01 (parity enforcement), CMP-02 (selection policy up front), CMP-03 (Pareto front), CMP-04 (dominated variant notes), CMP-06 (CL across options)
Retrieve: haft_query(action="fpf", query="CMP-01") | Full phase: haft_query(action="fpf", query="compare pareto selection parity")
`,
	"decide": `
── FPF patterns for deciding ──
DEC-01 (record structure), DEC-04 (invariants), DEC-05 (rollback), DEC-06 (predictions), DEC-08 (counterargument preservation)
Retrieve: haft_query(action="fpf", query="DEC-01") | Full phase: haft_query(action="fpf", query="decide record invariant rollback")
`,
	"verify": `
── FPF patterns for verification ──
VER-01 (evidence graph), VER-02 (evidence decay), VER-03 (R_eff computation), VER-07 (refresh triggers), X-WLNK (weakest link principle)
Retrieve: haft_query(action="fpf", query="VER-01") | Full phase: haft_query(action="fpf", query="verify evidence decay refresh")
`,
}
