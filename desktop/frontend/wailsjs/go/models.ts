export namespace main {
	
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
	    status: string;
	    prompt: string;
	    branch: string;
	    worktree: boolean;
	    started_at: string;
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
	        this.status = source["status"];
	        this.prompt = source["prompt"];
	        this.branch = source["branch"];
	        this.worktree = source["worktree"];
	        this.started_at = source["started_at"];
	        this.output = source["output"];
	    }
	}
	

}

