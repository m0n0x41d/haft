# FPF Patterns: Explore Phase

## EXP-01: Abductive Loop
**Trigger:** Problem framed; generating plausible solutions
**Spec:** B.5.2, B.5.2.0

Abduction = inference to presently most plausible candidate. Four steps: (1) Frame the prompt (anomaly/opportunity/probe/cue), (2) Generate multiple rival candidates, (3) Apply plausibility filters, (4) Select and publish prime hypothesis. Maintain visibility of why each candidate was rejected or retained.

## EXP-02: Candidate Rivalry Preservation
**Trigger:** Landed on one appealing solution; worried about confirmation bias
**Spec:** B.5.2, B.5.2:4.4

Generate multiple candidate hypotheses (at least two), even if one looks favored early. Record the differentiating claim each candidate adds. Track whether each is live, deferred, or rejected. The most common failure: silent suppression of rivals — a polished record hides that alternatives were seriously considered.

## EXP-03: Plausibility Filters
**Trigger:** 2+ candidates; need to rank justifiably
**Spec:** B.5.2:4.4, B.5.2:16.2

Use at least two explicit filters: (1) Parsimony = only necessary structure? (2) Explanatory reach = how much of the prompt accounted for? (3) Consistency = avoids colliding with trusted constraints? (4) Falsifiability = enables deduction/testing? (5) Scope fit = framed for declared scope, not inflated? Record which filters favor which candidates.

## EXP-04: Weakest Link per Variant (WLNK)
**Trigger:** Variants look plausible; need each one's Achilles' heel
**Spec:** B.3.4.4

For each variant, ask: "What will break first if this is wrong?" The weakest link is the single component, assumption, or integration point most likely to fail. Naming WLNK makes risk transparent. Evidence-gathering focuses on testing WLNK first.

## EXP-05: Stepping Stone Identification
**Trigger:** Option A is best today but Option B keeps doors open
**Spec:** C.18

Some variants are stepping stones: not locally optimal but enable future discovery. Mark explicitly. Document why this variant is a stepping stone and what future possibilities it opens. Prevents portfolio from collapsing to greedy local choice.

## EXP-06: Novelty Marker
**Trigger:** Building a portfolio; ensuring variants explore different dimensions
**Source:** Haft operational pattern

Every variant must have an explicit novelty marker: what is the substantive difference from other options? Different mechanism? Different tradeoff? Different scope? Surface rephrasing is not novelty. Genuine diversity means materially different solution spaces.

## EXP-07: Portfolio Thinking on Pareto Fronts
**Trigger:** Multiple candidates or variant solutions
**Source:** Haft operational pattern (derived from engineering practice)

Don't pick one winner; hold a set of non-dominated solutions (Pareto-incomparable tradeoffs). Pareto fronts shift as you add new dimensions. Use NQD (Novelty-Quality-Diversity) to guide stepping stone selection, not just quality. Allocate 1-2 "entrepreneur bets" to stepping stones even if they score lower.

## EXP-08: New-Question Detection (NQD)
**Trigger:** Problem feels novel; existing taxonomies don't fit
**Spec:** C.18, C.18.1, B.5.2.0

Ask: Is this problem actually asking something solved before, or genuinely new? If new, existing templates may mislead. Document what makes this question new (different domain, constraints, scale, stakeholder class). Prevents defaulting to familiar but inapplicable solutions.
