package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	// "time" // Unused

	"database/sql" // Added

	"quint-mcp/db"
)

// CLI Flags
var (
	modeFlag   = flag.String("mode", "cli", "Mode: 'cli' or 'server'")
	roleFlag   = flag.String("role", "", "Role: Abductor, Deductor, Inductor, Auditor, Decider")
	actionFlag = flag.String("action", "check", "Action: check, transition, init, propose, evidence, loopback, decide, context, actualize, decay, audit")
	targetFlag = flag.String("target", "", "Target phase for transition")
	
	// Role Assignment Flags
	sessionIDFlag = flag.String("session_id", "default-session", "Session ID for the holder")
	contextFlag   = flag.String("context", "default-context", "Bounded Context")

	// Evidence Flags
	evidenceTypeFlag = flag.String("evidence_type", "", "Evidence type for transition anchor")
	evidenceURIFlag  = flag.String("evidence_uri", "", "URI/Path to evidence artifact")
	evidenceDescFlag = flag.String("evidence_desc", "", "Description of evidence")
evidenceHolonFlag = flag.String("evidence_holon", "", "Holon ID for assurance check") // Added for B.3

	// Tool Arguments
	titleFlag    = flag.String("title", "", "Title for hypothesis or decision")
	contentFlag  = flag.String("content", "", "Content body")
	typeFlag     = flag.String("type", "", "Evidence type (internal/external/logic)")
	targetIDFlag = flag.String("target_id", "", "Target ID for evidence (e.g. hypothesis filename)")
	verdictFlag  = flag.String("verdict", "", "Verdict (PASS/FAIL/REFINE)")
	insightFlag  = flag.String("insight", "", "Insight for loopback")

	// Extended Argument Flags
	scopeFlag             = flag.String("scope", "", "Scope for hypothesis (USM)")
	kindFlag              = flag.String("kind", "system", "Kind for hypothesis (system/episteme)")
	evidenceActionFlag    = flag.String("evidence_action", "add", "Action for evidence (add/check)")
	assuranceFlag         = flag.String("assurance", "", "Assurance level (L0/L1/L2)")
	carrierFlag           = flag.String("carrier", "", "Carrier reference")
	validUntilFlag        = flag.String("valid_until", "", "Validity expiration")
	drrContextFlag        = flag.String("drr_context", "", "DRR Context field")
	drrDecisionFlag       = flag.String("drr_decision", "", "DRR Decision field")
	drrRationaleFlag      = flag.String("drr_rationale", "", "DRR Rationale field")
	drrConsequencesFlag   = flag.String("drr_consequences", "", "DRR Consequences field")
	drrCharacteristicsFlag = flag.String("drr_characteristics", "", "DRR Characteristics field")
)

func main() {
	flag.Parse()

	// Locate .quint directory
	cwd, _ := os.Getwd()
	quintDir := filepath.Join(cwd, ".quint")
	stateFile := filepath.Join(quintDir, "state.json")
	dbPath := filepath.Join(quintDir, "quint.db")

	// Ensure .quint exists for init
	if *actionFlag == "init" {
		if err := os.MkdirAll(quintDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating .quint directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Initialize DB
	var database *db.DB
	if _, err := os.Stat(dbPath); err == nil || *actionFlag == "init" {
		var err error
		database, err = db.New(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to init DB: %v\n", err)
		}
	}

	// Load State
	var sqlDB *sql.DB
	if database != nil {
		sqlDB = database.GetRawDB()
	}

	fsm, err := LoadState(stateFile, sqlDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	tools := NewTools(fsm, cwd, database)

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
			HolonID:     *evidenceHolonFlag, // B.3 Assurance Check
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
		if !ok {
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
		assign := getRoleAssignment()
		ok, msg := fsm.CanTransition(PhaseAbduction, assign, nil)
		if !ok {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		path, err := tools.ProposeHypothesis(*titleFlag, *contentFlag, *scopeFlag, *kindFlag, "{}")
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SUCCESS: Created hypothesis %s\n", path)

	case "evidence":
		if !isValidRoleForPhase(fsm.State.Phase, Role(*roleFlag)) {
			fmt.Printf("DENIED: Role %s cannot add evidence in %s phase\n", *roleFlag, fsm.State.Phase)
			os.Exit(1)
		}
		path, err := tools.ManageEvidence(fsm.State.Phase, *evidenceActionFlag, *targetIDFlag, *typeFlag, *contentFlag, *verdictFlag, *assuranceFlag, *carrierFlag, *validUntilFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SUCCESS: Added evidence %s\n", path)

	case "loopback":
		assign := getRoleAssignment()
		evidence := &EvidenceStub{Type: "insight", Description: *insightFlag, URI: "loopback-event"}
		
		ok, msg := fsm.CanTransition(PhaseDeduction, assign, evidence)
		if !ok {
			fmt.Printf("DENIED: %s\n", msg)
			os.Exit(1)
		}
		childPath, err := tools.RefineLoopback(fsm.State.Phase, *targetIDFlag, *insightFlag, *titleFlag, *contentFlag, *scopeFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fsm.State.Phase = PhaseDeduction
		fsm.State.ActiveRole = assign
		if err := fsm.SaveState(stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state after loopback: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("LOOPBACK: Reset to DEDUCTION. Created refined hypothesis %s\n", childPath)

	case "decide":
		assign := getRoleAssignment()
		if fsm.State.Phase == PhaseInduction || fsm.State.Phase == PhaseAudit {
			evidence := getEvidenceStub()
			if evidence == nil {
				evidence = &EvidenceStub{Type: "rationale", Description: "Final decision rationale", URI: "decision-process"}
			}
			ok, msg := fsm.CanTransition(PhaseDecision, assign, evidence)
			if !ok {
				fmt.Printf("DENIED: %s\n", msg)
				os.Exit(1)
			}
			fsm.State.Phase = PhaseDecision
		}
		
		path, err := tools.FinalizeDecision(*titleFlag, *targetIDFlag, *drrContextFlag, *drrDecisionFlag, *drrRationaleFlag, *drrConsequencesFlag, *drrCharacteristicsFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
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

	case "decay":
		if err := tools.RunDecay(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("DECAY: Assurance scores updated.")

	case "audit":
		tree, err := tools.VisualizeAudit(*targetIDFlag)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(tree)
	}
}
