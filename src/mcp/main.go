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
	actionFlag = flag.String("action", "check", "Action: check, transition, init, propose, evidence, loopback, decide")
	targetFlag = flag.String("target", "", "Target phase for transition")
	
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

	// Locate .fpf directory
	cwd, _ := os.Getwd()
	fpfDir := filepath.Join(cwd, ".fpf")
	stateFile := filepath.Join(fpfDir, "state.json")

	// Ensure .fpf exists for init
	if *actionFlag == "init" {
		os.MkdirAll(fpfDir, 0755)
	}

	fsm, err := LoadState(stateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	tools := NewTools(fsm, cwd)

	if *modeFlag == "server" {
		// TODO: Implement MCP JSON-RPC loop here
		fmt.Println("MCP Server mode not yet implemented")
		return
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
		ok, msg := fsm.CanTransition(Phase(*targetFlag), Role(*roleFlag))
		if ok {
			fsm.State.Phase = Phase(*targetFlag)
			fsm.SaveState(stateFile)
			fmt.Printf("TRANSITION: %s\n", msg)
			os.Exit(0)
		} else {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		
	case "init":
		fsm.State.Phase = PhaseAbduction
		fsm.SaveState(stateFile)
		tools.InitProject() // Helper to create dirs
		fmt.Println("Initialized FPF project in .fpf/")

	// --- Tool Actions ---

	case "propose":
		// Role: Abductor. Phase: ABDUCTION.
		ok, msg := fsm.CanTransition(PhaseAbduction, Role(*roleFlag))
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
		// We check if the role is valid for the CURRENT phase
		if !isValidRoleForPhase(fsm.State.Phase, Role(*roleFlag)) {
			fmt.Printf("DENIED: Role %s cannot add evidence in %s phase\n", *roleFlag, fsm.State.Phase)
			os.Exit(1)
		}
		path, err := tools.ManageEvidence(*targetIDFlag, *typeFlag, *contentFlag, *verdictFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SUCCESS: Added evidence %s\n", path)

	case "loopback":
		// Role: Inductor (triggers it). Phase: INDUCTION -> DEDUCTION.
		// Note: The FSM rule is INDUCTION -> DEDUCTION by Inductor/Deductor.
		ok, msg := fsm.CanTransition(PhaseDeduction, Role(*roleFlag))
		if !ok {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		// Actually perform the loopback logic
		childPath, err := tools.RefineLoopback(*targetIDFlag, *insightFlag, *titleFlag, *contentFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		// Perform state transition
		fsm.State.Phase = PhaseDeduction
		fsm.SaveState(stateFile)
		fmt.Printf("LOOPBACK: Reset to DEDUCTION. Created refined hypothesis %s\n", childPath)

	case "decide":
		// Role: Decider. Phase: DECISION -> IDLE.
		// First transition to Decision if not already (from Induction)
		if fsm.State.Phase == PhaseInduction {
			ok, msg := fsm.CanTransition(PhaseDecision, Role(*roleFlag))
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
		fsm.SaveState(stateFile)
		fmt.Printf("DECIDED: DRR created at %s. Cycle closed.\n", path)
	}
}
