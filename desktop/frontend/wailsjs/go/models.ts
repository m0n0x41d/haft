export namespace main {
	
	export class AgentPreset {
	    name: string;
	    agent_kind: string;
	    model: string;
	    role: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.agent_kind = source["agent_kind"];
	        this.model = source["model"];
	        this.role = source["role"];
	    }
	}
	export class ArtifactView {
	    id: string;
	    kind: string;
	    title: string;
	    status: string;
	    mode: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ArtifactView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class NormRuleView {
	    dimension: string;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new NormRuleView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.method = source["method"];
	    }
	}
	export class ParityPlanView {
	    baseline_set: string[];
	    window: string;
	    budget: string;
	    normalization: NormRuleView[];
	    missing_data_policy: string;
	    pinned_conditions: string[];
	
	    static createFrom(source: any = {}) {
	        return new ParityPlanView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseline_set = source["baseline_set"];
	        this.window = source["window"];
	        this.budget = source["budget"];
	        this.normalization = this.convertValues(source["normalization"], NormRuleView);
	        this.missing_data_policy = source["missing_data_policy"];
	        this.pinned_conditions = source["pinned_conditions"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DimensionView {
	    name: string;
	    scale_type: string;
	    unit: string;
	    polarity: string;
	    role: string;
	    how_to_measure: string;
	    valid_until: string;
	
	    static createFrom(source: any = {}) {
	        return new DimensionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.scale_type = source["scale_type"];
	        this.unit = source["unit"];
	        this.polarity = source["polarity"];
	        this.role = source["role"];
	        this.how_to_measure = source["how_to_measure"];
	        this.valid_until = source["valid_until"];
	    }
	}
	export class CharacterizationView {
	    version: number;
	    dimensions: DimensionView[];
	    parity_plan?: ParityPlanView;
	
	    static createFrom(source: any = {}) {
	        return new CharacterizationView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.dimensions = this.convertValues(source["dimensions"], DimensionView);
	        this.parity_plan = this.convertValues(source["parity_plan"], ParityPlanView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ClaimView {
	    id: string;
	    claim: string;
	    observable: string;
	    threshold: string;
	    status: string;
	    verify_after: string;
	
	    static createFrom(source: any = {}) {
	        return new ClaimView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.claim = source["claim"];
	        this.observable = source["observable"];
	        this.threshold = source["threshold"];
	        this.status = source["status"];
	        this.verify_after = source["verify_after"];
	    }
	}
	export class ComparisonDimensionInput {
	    name: string;
	    scale_type: string;
	    unit: string;
	    polarity: string;
	    role: string;
	    how_to_measure: string;
	    valid_until: string;
	
	    static createFrom(source: any = {}) {
	        return new ComparisonDimensionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.scale_type = source["scale_type"];
	        this.unit = source["unit"];
	        this.polarity = source["polarity"];
	        this.role = source["role"];
	        this.how_to_measure = source["how_to_measure"];
	        this.valid_until = source["valid_until"];
	    }
	}
	export class TradeoffNote {
	    variant: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new TradeoffNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant = source["variant"];
	        this.summary = source["summary"];
	    }
	}
	export class DominatedNote {
	    variant: string;
	    dominated_by: string[];
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new DominatedNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant = source["variant"];
	        this.dominated_by = source["dominated_by"];
	        this.summary = source["summary"];
	    }
	}
	export class ComparisonView {
	    dimensions: string[];
	    scores: Record<string, any>;
	    non_dominated_set: string[];
	    dominated_notes: DominatedNote[];
	    pareto_tradeoffs: TradeoffNote[];
	    policy_applied: string;
	    selected_ref: string;
	    recommendation: string;
	
	    static createFrom(source: any = {}) {
	        return new ComparisonView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimensions = source["dimensions"];
	        this.scores = source["scores"];
	        this.non_dominated_set = source["non_dominated_set"];
	        this.dominated_notes = this.convertValues(source["dominated_notes"], DominatedNote);
	        this.pareto_tradeoffs = this.convertValues(source["pareto_tradeoffs"], TradeoffNote);
	        this.policy_applied = source["policy_applied"];
	        this.selected_ref = source["selected_ref"];
	        this.recommendation = source["recommendation"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CoverageModuleView {
	    id: string;
	    path: string;
	    name: string;
	    lang: string;
	    status: string;
	    decision_count: number;
	    decision_ids: string[];
	    impacted: boolean;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new CoverageModuleView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.lang = source["lang"];
	        this.status = source["status"];
	        this.decision_count = source["decision_count"];
	        this.decision_ids = source["decision_ids"];
	        this.impacted = source["impacted"];
	        this.files = source["files"];
	    }
	}
	export class CoverageView {
	    total_modules: number;
	    covered_count: number;
	    partial_count: number;
	    blind_count: number;
	    governed_percent: number;
	    last_scanned: string;
	    modules: CoverageModuleView[];
	
	    static createFrom(source: any = {}) {
	        return new CoverageView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_modules = source["total_modules"];
	        this.covered_count = source["covered_count"];
	        this.partial_count = source["partial_count"];
	        this.blind_count = source["blind_count"];
	        this.governed_percent = source["governed_percent"];
	        this.last_scanned = source["last_scanned"];
	        this.modules = this.convertValues(source["modules"], CoverageModuleView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DecisionView {
	    id: string;
	    title: string;
	    status: string;
	    mode: string;
	    selected_title: string;
	    weakest_link: string;
	    valid_until: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new DecisionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.selected_title = source["selected_title"];
	        this.weakest_link = source["weakest_link"];
	        this.valid_until = source["valid_until"];
	        this.created_at = source["created_at"];
	    }
	}
	export class ProblemView {
	    id: string;
	    title: string;
	    status: string;
	    mode: string;
	    signal: string;
	    reversibility: string;
	    constraints: string[];
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ProblemView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.signal = source["signal"];
	        this.reversibility = source["reversibility"];
	        this.constraints = source["constraints"];
	        this.created_at = source["created_at"];
	    }
	}
	export class DashboardView {
	    project_name: string;
	    problem_count: number;
	    decision_count: number;
	    portfolio_count: number;
	    note_count: number;
	    stale_count: number;
	    recent_problems: ProblemView[];
	    recent_decisions: DecisionView[];
	    stale_items: ArtifactView[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.project_name = source["project_name"];
	        this.problem_count = source["problem_count"];
	        this.decision_count = source["decision_count"];
	        this.portfolio_count = source["portfolio_count"];
	        this.note_count = source["note_count"];
	        this.stale_count = source["stale_count"];
	        this.recent_problems = this.convertValues(source["recent_problems"], ProblemView);
	        this.recent_decisions = this.convertValues(source["recent_decisions"], DecisionView);
	        this.stale_items = this.convertValues(source["stale_items"], ArtifactView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DecisionPredictionInput {
	    claim: string;
	    observable: string;
	    threshold: string;
	    verify_after: string;
	
	    static createFrom(source: any = {}) {
	        return new DecisionPredictionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.claim = source["claim"];
	        this.observable = source["observable"];
	        this.threshold = source["threshold"];
	        this.verify_after = source["verify_after"];
	    }
	}
	export class DecisionRollbackInput {
	    triggers: string[];
	    steps: string[];
	    blast_radius: string;
	
	    static createFrom(source: any = {}) {
	        return new DecisionRollbackInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.triggers = source["triggers"];
	        this.steps = source["steps"];
	        this.blast_radius = source["blast_radius"];
	    }
	}
	export class DecisionRejectionInput {
	    variant: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new DecisionRejectionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant = source["variant"];
	        this.reason = source["reason"];
	    }
	}
	export class DecisionCreateInput {
	    problem_ref: string;
	    problem_refs: string[];
	    portfolio_ref: string;
	    selected_ref: string;
	    selected_title: string;
	    why_selected: string;
	    selection_policy: string;
	    counterargument: string;
	    why_not_others: DecisionRejectionInput[];
	    invariants: string[];
	    pre_conditions: string[];
	    post_conditions: string[];
	    admissibility: string[];
	    evidence_requirements: string[];
	    rollback?: DecisionRollbackInput;
	    refresh_triggers: string[];
	    weakest_link: string;
	    valid_until: string;
	    context: string;
	    mode: string;
	    affected_files: string[];
	    predictions: DecisionPredictionInput[];
	    search_keywords: string;
	    first_module_coverage: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DecisionCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.problem_ref = source["problem_ref"];
	        this.problem_refs = source["problem_refs"];
	        this.portfolio_ref = source["portfolio_ref"];
	        this.selected_ref = source["selected_ref"];
	        this.selected_title = source["selected_title"];
	        this.why_selected = source["why_selected"];
	        this.selection_policy = source["selection_policy"];
	        this.counterargument = source["counterargument"];
	        this.why_not_others = this.convertValues(source["why_not_others"], DecisionRejectionInput);
	        this.invariants = source["invariants"];
	        this.pre_conditions = source["pre_conditions"];
	        this.post_conditions = source["post_conditions"];
	        this.admissibility = source["admissibility"];
	        this.evidence_requirements = source["evidence_requirements"];
	        this.rollback = this.convertValues(source["rollback"], DecisionRollbackInput);
	        this.refresh_triggers = source["refresh_triggers"];
	        this.weakest_link = source["weakest_link"];
	        this.valid_until = source["valid_until"];
	        this.context = source["context"];
	        this.mode = source["mode"];
	        this.affected_files = source["affected_files"];
	        this.predictions = this.convertValues(source["predictions"], DecisionPredictionInput);
	        this.search_keywords = source["search_keywords"];
	        this.first_module_coverage = source["first_module_coverage"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RejectionView {
	    variant: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new RejectionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant = source["variant"];
	        this.reason = source["reason"];
	    }
	}
	export class DecisionDetailView {
	    id: string;
	    title: string;
	    status: string;
	    mode: string;
	    problem_refs: string[];
	    selected_title: string;
	    why_selected: string;
	    selection_policy: string;
	    counterargument: string;
	    weakest_link: string;
	    why_not_others: RejectionView[];
	    invariants: string[];
	    pre_conditions: string[];
	    post_conditions: string[];
	    admissibility: string[];
	    evidence_requirements: string[];
	    refresh_triggers: string[];
	    claims: ClaimView[];
	    first_module_coverage: boolean;
	    affected_files: string[];
	    coverage_modules: CoverageModuleView[];
	    coverage_warnings: string[];
	    rollback_triggers: string[];
	    rollback_steps: string[];
	    rollback_blast_radius: string;
	    valid_until: string;
	    body: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new DecisionDetailView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.problem_refs = source["problem_refs"];
	        this.selected_title = source["selected_title"];
	        this.why_selected = source["why_selected"];
	        this.selection_policy = source["selection_policy"];
	        this.counterargument = source["counterargument"];
	        this.weakest_link = source["weakest_link"];
	        this.why_not_others = this.convertValues(source["why_not_others"], RejectionView);
	        this.invariants = source["invariants"];
	        this.pre_conditions = source["pre_conditions"];
	        this.post_conditions = source["post_conditions"];
	        this.admissibility = source["admissibility"];
	        this.evidence_requirements = source["evidence_requirements"];
	        this.refresh_triggers = source["refresh_triggers"];
	        this.claims = this.convertValues(source["claims"], ClaimView);
	        this.first_module_coverage = source["first_module_coverage"];
	        this.affected_files = source["affected_files"];
	        this.coverage_modules = this.convertValues(source["coverage_modules"], CoverageModuleView);
	        this.coverage_warnings = source["coverage_warnings"];
	        this.rollback_triggers = source["rollback_triggers"];
	        this.rollback_steps = source["rollback_steps"];
	        this.rollback_blast_radius = source["rollback_blast_radius"];
	        this.valid_until = source["valid_until"];
	        this.body = source["body"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class DesktopConfig {
	    default_agent: string;
	    review_agent: string;
	    verify_agent: string;
	    agent_presets: AgentPreset[];
	    task_timeout_minutes: number;
	    sound_enabled: boolean;
	    notify_enabled: boolean;
	    default_ide: string;
	    default_worktree: boolean;
	    auto_wire_mcp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DesktopConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default_agent = source["default_agent"];
	        this.review_agent = source["review_agent"];
	        this.verify_agent = source["verify_agent"];
	        this.agent_presets = this.convertValues(source["agent_presets"], AgentPreset);
	        this.task_timeout_minutes = source["task_timeout_minutes"];
	        this.sound_enabled = source["sound_enabled"];
	        this.notify_enabled = source["notify_enabled"];
	        this.default_ide = source["default_ide"];
	        this.default_worktree = source["default_worktree"];
	        this.auto_wire_mcp = source["auto_wire_mcp"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DesktopFlow {
	    id: string;
	    project_name: string;
	    project_path: string;
	    title: string;
	    description: string;
	    template_id: string;
	    agent: string;
	    prompt: string;
	    schedule: string;
	    branch: string;
	    use_worktree: boolean;
	    enabled: boolean;
	    last_task_id: string;
	    last_run_at: string;
	    next_run_at: string;
	    last_error: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopFlow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project_name = source["project_name"];
	        this.project_path = source["project_path"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.template_id = source["template_id"];
	        this.agent = source["agent"];
	        this.prompt = source["prompt"];
	        this.schedule = source["schedule"];
	        this.branch = source["branch"];
	        this.use_worktree = source["use_worktree"];
	        this.enabled = source["enabled"];
	        this.last_task_id = source["last_task_id"];
	        this.last_run_at = source["last_run_at"];
	        this.next_run_at = source["next_run_at"];
	        this.last_error = source["last_error"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	
	
	export class DominatedNoteInput {
	    variant: string;
	    dominated_by: string[];
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new DominatedNoteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant = source["variant"];
	        this.dominated_by = source["dominated_by"];
	        this.summary = source["summary"];
	    }
	}
	export class FlowInput {
	    id: string;
	    title: string;
	    description: string;
	    template_id: string;
	    agent: string;
	    prompt: string;
	    schedule: string;
	    branch: string;
	    use_worktree: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FlowInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.template_id = source["template_id"];
	        this.agent = source["agent"];
	        this.prompt = source["prompt"];
	        this.schedule = source["schedule"];
	        this.branch = source["branch"];
	        this.use_worktree = source["use_worktree"];
	        this.enabled = source["enabled"];
	    }
	}
	export class FlowTemplate {
	    id: string;
	    name: string;
	    description: string;
	    agent: string;
	    schedule: string;
	    prompt: string;
	    branch: string;
	    use_worktree: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FlowTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.agent = source["agent"];
	        this.schedule = source["schedule"];
	        this.prompt = source["prompt"];
	        this.branch = source["branch"];
	        this.use_worktree = source["use_worktree"];
	    }
	}
	export class GovernanceFindingView {
	    id: string;
	    artifact_ref: string;
	    title: string;
	    kind: string;
	    category: string;
	    reason: string;
	    valid_until: string;
	    days_stale: number;
	    r_eff: number;
	    drift_count: number;
	
	    static createFrom(source: any = {}) {
	        return new GovernanceFindingView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.artifact_ref = source["artifact_ref"];
	        this.title = source["title"];
	        this.kind = source["kind"];
	        this.category = source["category"];
	        this.reason = source["reason"];
	        this.valid_until = source["valid_until"];
	        this.days_stale = source["days_stale"];
	        this.r_eff = source["r_eff"];
	        this.drift_count = source["drift_count"];
	    }
	}
	export class ProblemCandidateView {
	    id: string;
	    status: string;
	    title: string;
	    signal: string;
	    acceptance: string;
	    context: string;
	    category: string;
	    source_artifact_ref: string;
	    source_title: string;
	    problem_ref: string;
	
	    static createFrom(source: any = {}) {
	        return new ProblemCandidateView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.title = source["title"];
	        this.signal = source["signal"];
	        this.acceptance = source["acceptance"];
	        this.context = source["context"];
	        this.category = source["category"];
	        this.source_artifact_ref = source["source_artifact_ref"];
	        this.source_title = source["source_title"];
	        this.problem_ref = source["problem_ref"];
	    }
	}
	export class GovernanceOverviewView {
	    last_scan_at: string;
	    coverage: CoverageView;
	    findings: GovernanceFindingView[];
	    problem_candidates: ProblemCandidateView[];
	
	    static createFrom(source: any = {}) {
	        return new GovernanceOverviewView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last_scan_at = source["last_scan_at"];
	        this.coverage = this.convertValues(source["coverage"], CoverageView);
	        this.findings = this.convertValues(source["findings"], GovernanceFindingView);
	        this.problem_candidates = this.convertValues(source["problem_candidates"], ProblemCandidateView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InstalledAgent {
	    kind: string;
	    name: string;
	    path: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new InstalledAgent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
	    }
	}
	export class NormRuleInput {
	    dimension: string;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new NormRuleInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.method = source["method"];
	    }
	}
	
	export class ParityPlanInput {
	    baseline_set: string[];
	    window: string;
	    budget: string;
	    normalization: NormRuleInput[];
	    missing_data_policy: string;
	    pinned_conditions: string[];
	
	    static createFrom(source: any = {}) {
	        return new ParityPlanInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseline_set = source["baseline_set"];
	        this.window = source["window"];
	        this.budget = source["budget"];
	        this.normalization = this.convertValues(source["normalization"], NormRuleInput);
	        this.missing_data_policy = source["missing_data_policy"];
	        this.pinned_conditions = source["pinned_conditions"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class TradeoffNoteInput {
	    variant: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new TradeoffNoteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant = source["variant"];
	        this.summary = source["summary"];
	    }
	}
	export class PortfolioCompareInput {
	    portfolio_ref: string;
	    dimensions: string[];
	    scores: Record<string, any>;
	    non_dominated_set: string[];
	    incomparable: string[][];
	    dominated_notes: DominatedNoteInput[];
	    pareto_tradeoffs: TradeoffNoteInput[];
	    policy_applied: string;
	    selected_ref: string;
	    recommendation: string;
	    parity_plan?: ParityPlanInput;
	
	    static createFrom(source: any = {}) {
	        return new PortfolioCompareInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.portfolio_ref = source["portfolio_ref"];
	        this.dimensions = source["dimensions"];
	        this.scores = source["scores"];
	        this.non_dominated_set = source["non_dominated_set"];
	        this.incomparable = source["incomparable"];
	        this.dominated_notes = this.convertValues(source["dominated_notes"], DominatedNoteInput);
	        this.pareto_tradeoffs = this.convertValues(source["pareto_tradeoffs"], TradeoffNoteInput);
	        this.policy_applied = source["policy_applied"];
	        this.selected_ref = source["selected_ref"];
	        this.recommendation = source["recommendation"];
	        this.parity_plan = this.convertValues(source["parity_plan"], ParityPlanInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PortfolioVariantInput {
	    id: string;
	    title: string;
	    description: string;
	    strengths: string[];
	    weakest_link: string;
	    novelty_marker: string;
	    risks: string[];
	    stepping_stone: boolean;
	    stepping_stone_basis: string;
	    diversity_role: string;
	    assumption_notes: string;
	    rollback_notes: string;
	    evidence_refs: string[];
	
	    static createFrom(source: any = {}) {
	        return new PortfolioVariantInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.strengths = source["strengths"];
	        this.weakest_link = source["weakest_link"];
	        this.novelty_marker = source["novelty_marker"];
	        this.risks = source["risks"];
	        this.stepping_stone = source["stepping_stone"];
	        this.stepping_stone_basis = source["stepping_stone_basis"];
	        this.diversity_role = source["diversity_role"];
	        this.assumption_notes = source["assumption_notes"];
	        this.rollback_notes = source["rollback_notes"];
	        this.evidence_refs = source["evidence_refs"];
	    }
	}
	export class PortfolioCreateInput {
	    problem_ref: string;
	    context: string;
	    mode: string;
	    no_stepping_stone_rationale: string;
	    variants: PortfolioVariantInput[];
	
	    static createFrom(source: any = {}) {
	        return new PortfolioCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.problem_ref = source["problem_ref"];
	        this.context = source["context"];
	        this.mode = source["mode"];
	        this.no_stepping_stone_rationale = source["no_stepping_stone_rationale"];
	        this.variants = this.convertValues(source["variants"], PortfolioVariantInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class VariantView {
	    id: string;
	    title: string;
	    description: string;
	    weakest_link: string;
	    novelty_marker: string;
	    stepping_stone: boolean;
	    strengths: string[];
	    risks: string[];
	
	    static createFrom(source: any = {}) {
	        return new VariantView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.weakest_link = source["weakest_link"];
	        this.novelty_marker = source["novelty_marker"];
	        this.stepping_stone = source["stepping_stone"];
	        this.strengths = source["strengths"];
	        this.risks = source["risks"];
	    }
	}
	export class PortfolioDetailView {
	    id: string;
	    title: string;
	    status: string;
	    problem_ref: string;
	    variants: VariantView[];
	    comparison?: ComparisonView;
	    body: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new PortfolioDetailView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.problem_ref = source["problem_ref"];
	        this.variants = this.convertValues(source["variants"], VariantView);
	        this.comparison = this.convertValues(source["comparison"], ComparisonView);
	        this.body = source["body"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PortfolioSummaryView {
	    id: string;
	    title: string;
	    status: string;
	    mode: string;
	    problem_ref: string;
	    has_comparison: boolean;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new PortfolioSummaryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.problem_ref = source["problem_ref"];
	        this.has_comparison = source["has_comparison"];
	        this.created_at = source["created_at"];
	    }
	}
	
	
	export class ProblemCharacterizationInput {
	    problem_ref: string;
	    dimensions: ComparisonDimensionInput[];
	    parity_rules: string;
	    parity_plan?: ParityPlanInput;
	
	    static createFrom(source: any = {}) {
	        return new ProblemCharacterizationInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.problem_ref = source["problem_ref"];
	        this.dimensions = this.convertValues(source["dimensions"], ComparisonDimensionInput);
	        this.parity_rules = source["parity_rules"];
	        this.parity_plan = this.convertValues(source["parity_plan"], ParityPlanInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProblemCreateInput {
	    title: string;
	    signal: string;
	    acceptance: string;
	    blast_radius: string;
	    reversibility: string;
	    context: string;
	    mode: string;
	    constraints: string[];
	    optimization_targets: string[];
	    observation_indicators: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProblemCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.signal = source["signal"];
	        this.acceptance = source["acceptance"];
	        this.blast_radius = source["blast_radius"];
	        this.reversibility = source["reversibility"];
	        this.context = source["context"];
	        this.mode = source["mode"];
	        this.constraints = source["constraints"];
	        this.optimization_targets = source["optimization_targets"];
	        this.observation_indicators = source["observation_indicators"];
	    }
	}
	export class ProblemDetailView {
	    id: string;
	    title: string;
	    status: string;
	    mode: string;
	    signal: string;
	    constraints: string[];
	    optimization_targets: string[];
	    observation_indicators: string[];
	    acceptance: string;
	    blast_radius: string;
	    reversibility: string;
	    characterizations: CharacterizationView[];
	    latest_characterization?: CharacterizationView;
	    linked_portfolios: ArtifactView[];
	    linked_decisions: ArtifactView[];
	    body: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ProblemDetailView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.signal = source["signal"];
	        this.constraints = source["constraints"];
	        this.optimization_targets = source["optimization_targets"];
	        this.observation_indicators = source["observation_indicators"];
	        this.acceptance = source["acceptance"];
	        this.blast_radius = source["blast_radius"];
	        this.reversibility = source["reversibility"];
	        this.characterizations = this.convertValues(source["characterizations"], CharacterizationView);
	        this.latest_characterization = this.convertValues(source["latest_characterization"], CharacterizationView);
	        this.linked_portfolios = this.convertValues(source["linked_portfolios"], ArtifactView);
	        this.linked_decisions = this.convertValues(source["linked_decisions"], ArtifactView);
	        this.body = source["body"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProjectInfo {
	    path: string;
	    name: string;
	    id: string;
	    is_active: boolean;
	    problem_count: number;
	    decision_count: number;
	    stale_count: number;
	
	    static createFrom(source: any = {}) {
	        return new ProjectInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.id = source["id"];
	        this.is_active = source["is_active"];
	        this.problem_count = source["problem_count"];
	        this.decision_count = source["decision_count"];
	        this.stale_count = source["stale_count"];
	    }
	}
	
	export class TaskState {
	    id: string;
	    title: string;
	    agent: string;
	    project: string;
	    project_path: string;
	    status: string;
	    prompt: string;
	    branch: string;
	    worktree: boolean;
	    worktree_path: string;
	    reused_worktree: boolean;
	    started_at: string;
	    completed_at: string;
	    error_message: string;
	    output: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.agent = source["agent"];
	        this.project = source["project"];
	        this.project_path = source["project_path"];
	        this.status = source["status"];
	        this.prompt = source["prompt"];
	        this.branch = source["branch"];
	        this.worktree = source["worktree"];
	        this.worktree_path = source["worktree_path"];
	        this.reused_worktree = source["reused_worktree"];
	        this.started_at = source["started_at"];
	        this.completed_at = source["completed_at"];
	        this.error_message = source["error_message"];
	        this.output = source["output"];
	    }
	}
	export class TerminalSession {
	    id: string;
	    title: string;
	    project_path: string;
	    cwd: string;
	    shell: string;
	    status: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.project_path = source["project_path"];
	        this.cwd = source["cwd"];
	        this.shell = source["shell"];
	        this.status = source["status"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	
	

}

