package fpf

// SourceAccessPublications adds the ecosystem's own usage guide to the bounded
// Engineering corpus. It does not import the other suites linked by the guide.
func SourceAccessPublications() []PublicationDescriptor {
	guide := PublicationDescriptor{
		ID: "fpf-usage-guide", Kind: "usage_guide", Namespace: "FPF-USAGE",
		Path: "USING-FPF.md", Root: "Using FPF and its DPF Suites",
	}
	publications := []PublicationDescriptor{guide}
	engineering := EngineeringPublications()
	return append(publications, engineering...)
}

// EngineeringPublications is the explicit supported publication set. Its order
// is deterministic and conveys no application or work order.
func EngineeringPublications() []PublicationDescriptor {
	return []PublicationDescriptor{
		{ID: "engineering-suite", Kind: "suite_navigation", Namespace: "ENGINEERING-SUITE", Path: "Engineering DPF Suite/README.md", Root: "Engineering DPF Suite"},
		{ID: "engineering-suite-reference", Kind: "suite_navigation", Namespace: "ENGINEERING-REFERENCE", Path: "Engineering DPF Suite/ENGINEERING-DPF-SUITE-REFERENCE.md", Root: "Engineering DPF Suite Reference"},
		{ID: "dpf-fin", Kind: "engineering_dpf", Namespace: "FIN", Path: "Engineering DPF Suite/CORPORATE-FINANCE-PRINCIPLES-FRAMEWORK.md", Root: "Corporate Finance Principles Framework"},
		{ID: "dpf-cgov", Kind: "engineering_dpf", Namespace: "CGOV", Path: "Engineering DPF Suite/CORPORATE-GOVERNANCE-PRINCIPLES-FRAMEWORK.md", Root: "Corporate Governance Principles Framework"},
		{ID: "dpf-doca", Kind: "engineering_dpf", Namespace: "DOCA", Path: "Engineering DPF Suite/DEVELOPMENT-OPPORTUNITY-CONSTRUCTION-AND-DEVELOPMENT-DIRECTION-ADVISING-PRINCIPLES-FRAMEWORK.md", Root: "Development Opportunity Construction and Development-Direction Advising Principles Framework"},
		{ID: "dpf-eco", Kind: "engineering_dpf", Namespace: "ECO", Path: "Engineering DPF Suite/ECONOMIC-REASONING-AND-COORDINATION-PRINCIPLES-FRAMEWORK.md", Root: "Economic Reasoning and Coordination Principles Framework"},
		{ID: "dpf-rhy", Kind: "engineering_dpf", Namespace: "RHY", Path: "Engineering DPF Suite/EMBODIED-RHYTHMICS-PRINCIPLES-FRAMEWORK.md", Root: "Embodied Rhythmics Principles Framework"},
		{ID: "dpf-eam", Kind: "engineering_dpf", Namespace: "EAM", Path: "Engineering DPF Suite/ENGINEERING-ASSET-MANAGEMENT-PRINCIPLES-FRAMEWORK.md", Root: "Engineering Asset Management Principles Framework"},
		{ID: "dpf-exd", Kind: "engineering_dpf", Namespace: "EXD", Path: "Engineering DPF Suite/EXPLANATION-DESIGN-PRINCIPLES-FRAMEWORK.md", Root: "Explanation Design Principles Framework"},
		{ID: "dpf-fdm", Kind: "engineering_dpf", Namespace: "FDM", Path: "Engineering DPF Suite/FINANCIAL-DOMAIN-MODELING-PRINCIPLES-FRAMEWORK.md", Root: "Financial Domain Modeling Principles Framework"},
		{ID: "dpf-hcd", Kind: "engineering_dpf", Namespace: "HCD", Path: "Engineering DPF Suite/HUMAN-CAPABILITY-DEVELOPMENT-PRINCIPLES-FRAMEWORK.md", Root: "Human Capability Development Principles Framework"},
		{ID: "dpf-mnt", Kind: "engineering_dpf", Namespace: "MNT", Path: "Engineering DPF Suite/MAINTENANCE-ENGINEERING-PRINCIPLES-FRAMEWORK.md", Root: "Maintenance Engineering and Management Principles Framework"},
		{ID: "dpf-ma", Kind: "engineering_dpf", Namespace: "MA", Path: "Engineering DPF Suite/MANAGEMENT-ACCOUNTING-PRINCIPLES-FRAMEWORK.md", Root: "Management Accounting Principles Framework"},
		{ID: "dpf-me", Kind: "engineering_dpf", Namespace: "ME", Path: "Engineering DPF Suite/METHOD-ENGINEERING-PRINCIPLES-FRAMEWORK.md", Root: "Method Engineering Principles Framework"},
		{ID: "dpf-mdpe", Kind: "engineering_dpf", Namespace: "MDPE", Path: "Engineering DPF Suite/MUSIC-AND-DANCE-PRACTICE-ENGINEERING-PRINCIPLES-FRAMEWORK.md", Root: "Music and Dance Practice Engineering Principles Framework"},
		{ID: "dpf-ops", Kind: "engineering_dpf", Namespace: "OPS", Path: "Engineering DPF Suite/OPERATIONS-MANAGEMENT-PRINCIPLES-FRAMEWORK.md", Root: "Operations Management Principles Framework"},
		{ID: "dpf-adm", Kind: "engineering_dpf", Namespace: "ADM", Path: "Engineering DPF Suite/ORGANIZATION-ADMINISTRATION-PRINCIPLES-FRAMEWORK.md", Root: "Organization Administration Principles Framework"},
		{ID: "dpf-oce", Kind: "engineering_dpf", Namespace: "OCE", Path: "Engineering DPF Suite/ORGANIZATION-CHANGE-ENGINEERING-PRINCIPLES-FRAMEWORK.md", Root: "Organization Change Engineering Principles Framework"},
		{ID: "dpf-psd", Kind: "engineering_dpf", Namespace: "PSD", Path: "Engineering DPF Suite/PROBLEM-STRUCTURING-AND-DECISION-SUPPORT-PRINCIPLES-FRAMEWORK.md", Root: "Problem Structuring and Decision Support Principles Framework"},
		{ID: "dpf-rmp", Kind: "engineering_dpf", Namespace: "RMP", Path: "Engineering DPF Suite/RESEARCH-METHOD-PRACTICE-PRINCIPLES-FRAMEWORK.md", Root: "Research Method Practice Principles Framework"},
		{ID: "dpf-sie", Kind: "engineering_dpf", Namespace: "SIE", Path: "Engineering DPF Suite/SEMANTIC-INTEGRATION-ENGINEERING-PRINCIPLES-FRAMEWORK.md", Root: "Semantic Integration Engineering Principles Framework"},
		{ID: "dpf-str", Kind: "engineering_dpf", Namespace: "STR", Path: "Engineering DPF Suite/STRATEGY-PRINCIPLES-FRAMEWORK.md", Root: "Strategy Principles Framework"},
		{ID: "dpf-syse", Kind: "engineering_dpf", Namespace: "SYSE", Path: "Engineering DPF Suite/SYSTEMS-ENGINEERING-PRINCIPLES-FRAMEWORK.md", Root: "Systems Engineering Principles Framework"},
	}
}
