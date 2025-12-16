package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// CLI Flags
var (
	modeFlag   = flag.String("mode", "cli", "Mode: 'cli' or 'server'")
	roleFlag   = flag.String("role", "", "Role: Abductor, Deductor, Inductor, Auditor, Decider")
	actionFlag = flag.String("action", "check", "Action: check, transition, init, propose, evidence, loopback, decide, context, actualize")
	targetFlag = flag.String("target", "", "Target phase for transition")
	
	// Role Assignment Flags
	sessionIDFlag = flag.String("session_id", "default-session", "Session ID for the holder")
	contextFlag   = flag.String("context", "default-context", "Bounded Context")

	// Evidence Flags
	evidenceTypeFlag = flag.String("evidence_type", "", "Evidence type for transition anchor")
	evidenceURIFlag  = flag.String("evidence_uri", "", "URI/Path to evidence artifact")
	evidenceDescFlag = flag.String("evidence_desc", "", "Description of evidence")

	// Tool Arguments
	titleFlag    = flag.String("title", "", "Title for hypothesis or decision")
	contentFlag  = flag.String("content", "", "Content body")
	typeFlag     = flag.String("type", "", "Evidence type (internal/external/logic)")
	targetIDFlag = flag.String("target_id", "", "Target ID for evidence (e.g. hypothesis filename)")
	verdictFlag  = flag.String("verdict", "", "Verdict (PASS/FAIL/REFINE)")
	insightFlag  = flag.String("insight", "", "Insight for loopback")
)

func main() {
	flag.Parse()

	// Locate .quint directory
	cwd, _ := os.Getwd()
	quintDir := filepath.Join(cwd, ".quint")
	stateFile := filepath.Join(quintDir, "state.json")

	// Ensure .quint exists for init
	if *actionFlag == "init" {
		if err := os.MkdirAll(quintDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating .quint directory: %v\n", err)
			os.Exit(1)
		}
	}

	fsm, err := LoadState(stateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	tools := NewTools(fsm, cwd)

	if *modeFlag == "server" {
		server := NewServer(tools)
		server.Start()
		return
	}

	// Helper to construct RoleAssignment
	getRoleAssignment := func() RoleAssignment {
		return RoleAssignment{
			Role:      Role(*roleFlag),
			SessionID: *sessionIDFlag,
			Context:   *contextFlag,
		}
	}

	// Helper to construct EvidenceStub (returns nil if empty)
	getEvidenceStub := func() *EvidenceStub {
		if *evidenceURIFlag == "" {
			return nil
		}
		return &EvidenceStub{
			Type:        *evidenceTypeFlag,
			URI:         *evidenceURIFlag,
			Description: *evidenceDescFlag,
		}
	}

	// CLI Mode
	switch *actionFlag {
	case "status":
		fmt.Println(fsm.State.Phase)
		
	case "check":
		if *roleFlag == "" {
			fmt.Println("Error: --role required")
			os.Exit(1)
		}
		// Check if role acts in current phase
		if isValidRoleForPhase(fsm.State.Phase, Role(*roleFlag)) {
			fmt.Printf("OK: %s active in %s\n", *roleFlag, fsm.State.Phase)
			os.Exit(0)
		} else {
			fmt.Printf("VIOLATION: %s cannot act in %s\n", *roleFlag, fsm.State.Phase)
			os.Exit(1)
		}

	case "transition":
		if *targetFlag == "" || *roleFlag == "" {
			fmt.Println("Error: --target and --role required")
			os.Exit(1)
		}
		
		assign := getRoleAssignment()
		evidence := getEvidenceStub()

		ok, msg := fsm.CanTransition(Phase(*targetFlag), assign, evidence)
		if ok {
			fsm.State.Phase = Phase(*targetFlag)
			fsm.State.ActiveRole = assign
			if err := fsm.SaveState(stateFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("TRANSITION: %s\n", msg)
			os.Exit(0)
		} else {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		
	case "init":
		fsm.State.Phase = PhaseAbduction
		if err := fsm.SaveState(stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			os.Exit(1)
		}
		if err := tools.InitProject(); err != nil { // Helper to create dirs
			fmt.Fprintf(os.Stderr, "Error initializing project: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Initialized FPF project in .quint/")

	// --- Tool Actions ---

	case "context":
		if *roleFlag == "" {
			fmt.Println("Error: --role required")
			os.Exit(1)
		}
		ctx, err := tools.GetAgentContext(*roleFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(ctx)

	case "propose":
		// Role: Abductor. Phase: ABDUCTION.
		// For propose (Abduction), no antecedent evidence is STRICTLY required to enter,
		// but we check transition to ensure we are in Abduction or can start it.
		// However, Propose is an ACTION within Abduction.
		
		assign := getRoleAssignment()
		// No evidence needed to just 'be' in Abduction if already there.
		// But if we were transitioning TO abduction, we might need it? No, Init handles that.
		
		// We re-check transition logic just to validate Role vs Phase permissions
		ok, msg := fsm.CanTransition(PhaseAbduction, assign, nil)
		if !ok {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		path, err := tools.ProposeHypothesis(*titleFlag, *contentFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SUCCESS: Created hypothesis %s\n", path)

	case "evidence":
		// Role: Deductor (DEDUCTION) or Inductor (INDUCTION)
		if !isValidRoleForPhase(fsm.State.Phase, Role(*roleFlag)) {
			fmt.Printf("DENIED: Role %s cannot add evidence in %s phase\n", *roleFlag, fsm.State.Phase)
			os.Exit(1)
		}
		path, err := tools.ManageEvidence(fsm.State.Phase, *targetIDFlag, *typeFlag, *contentFlag, *verdictFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SUCCESS: Added evidence %s\n", path)

	case "loopback":
		// Role: Inductor (triggers it). Phase: INDUCTION -> DEDUCTION.
		assign := getRoleAssignment()
		// Loopback implies we have 'failed' evidence (the insight).
		// We treat the insight as the evidence for this transition back.
		evidence := &EvidenceStub{Type: "insight", Description: *insightFlag, URI: "loopback-event"}
		
		ok, msg := fsm.CanTransition(PhaseDeduction, assign, evidence)
		if !ok {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		// Actually perform the loopback logic
		childPath, err := tools.RefineLoopback(fsm.State.Phase, *targetIDFlag, *insightFlag, *titleFlag, *contentFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		// Perform state transition
		fsm.State.Phase = PhaseDeduction
		fsm.State.ActiveRole = assign
		if err := fsm.SaveState(stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state after loopback: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("LOOPBACK: Reset to DEDUCTION. Created refined hypothesis %s\n", childPath)

	case "decide":
		// Role: Decider. Phase: DECISION -> IDLE.
		assign := getRoleAssignment()
		
		// First transition to Decision if not already (from Induction)
		if fsm.State.Phase == PhaseInduction {
			// We need evidence to enter Decision (e.g. Validation Results)
			// For CLI, we accept an explicit flag or assume the 'content' contains the rationale/evidence
			evidence := getEvidenceStub()
			if evidence == nil {
				// Fallback: use current op as evidence
				evidence = &EvidenceStub{Type: "rationale", Description: "Final decision rationale", URI: "decision-process"}
			}
			
			ok, msg := fsm.CanTransition(PhaseDecision, assign, evidence)
			if !ok {
				fmt.Printf("DENIED: %s\n", msg)
				os.Exit(1)
			}
			fsm.State.Phase = PhaseDecision
		}
		
		path, err := tools.FinalizeDecision(*titleFlag, *contentFlag, *targetIDFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		
		// Close cycle
		fsm.State.Phase = PhaseIdle
		fsm.State.ActiveRole = assign
		if err := fsm.SaveState(stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("DECIDED: DRR created at %s. Cycle closed.\n", path)

	case "actualize":
		if err := tools.Actualize(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("ACTUALIZATION: Complete.")
	}
}