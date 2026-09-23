package carrier

import "sort"

// ProposalHeads returns current proposed leaves in a known record lineage.
// It is an authoring surface, separate from Heads and live alias resolution:
// proposals never acquire governing status because an editor selects them.
func (p Projection) ProposalHeads(id string) []string {
	line, known := p.lineageByID[id]
	if !known {
		is := p.ByID[id]
		if len(is) == 0 {
			return nil
		}
		line = p.lineage[is[0]]
	}
	var refs []string
	for i, e := range p.Entries {
		if p.lineage[i] == line && e.State == Proposed {
			refs = append(refs, e.Ref)
		}
	}
	sort.Strings(refs)
	return refs
}
