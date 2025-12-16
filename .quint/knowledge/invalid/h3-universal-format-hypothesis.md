---
id: h3-universal-format
type: hypothesis
created: 2025-12-14T12:15:00Z
problem: multi-platform-repo-structure
status: invalid
invalidated: 2025-12-14T12:25:00Z
invalidation_reason: |
  Failed deduction: Contradicts "Simplicity" constraint.
  Adds schema, spec format, generators, npm build — significant complexity for 11 commands × 4 platforms.
  This is YAGNI — solves enterprise-scale problem for small-scale project.
  Learning: Universal spec formats make sense for 50+ commands, 10+ platforms, or when
  machine-readability has concrete use cases. None apply here.
formality: 5
novelty: Novel
complexity: High
author: Claude (generated), Human (to review)
scope:
  applies_to: "Projects wanting maximum portability with structured command definitions"
  not_valid_for: "Projects prioritizing simplicity over portability"
  scale: "Adds abstraction layer; all platforms generated from spec"
---

# Hypothesis: Universal Command Specification Format

## 1. The Method (Design-Time)

### Proposed Approach
Define a platform-agnostic command specification format (YAML or JSON) that captures the semantic content of each FPF command. Generate all platform-specific files from this spec. The spec becomes the source of truth; Markdown/TOML are build artifacts.

### Rationale
If we're going to support multiple platforms long-term, having a machine-readable spec enables tooling: validation, diffing, testing across platforms, potential IDE integrations. This is what mature multi-platform projects do (OpenAPI, protobuf, etc.).

### Implementation Steps
1. Define `command-spec.schema.yaml` with fields: name, description, arguments, prompt-template, platform-hints
2. Convert existing 11 commands to `spec/fpf-*.yaml`
3. Create generators: `generate-claude.js`, `generate-gemini.js`, `generate-codex.js`
4. Build step: `npm run build` generates all platform outputs
5. CI validates spec schema, runs generators, tests outputs

### Expected Capability
- Machine-readable command definitions
- Guaranteed consistency across platforms
- Enables future tooling (linting, testing, IDE support)
- Adding new platform = write one generator

## 2. The Validation (Run-Time)

### Plausibility Assessment

| Filter | Score | Justification |
|--------|-------|---------------|
| **Simplicity** | Low | Adds spec format, schema, generators, build tooling |
| **Explanatory Power** | High | Solves multi-platform completely |
| **Consistency** | Medium | Conflicts with "don't become bloated" constraint |
| **Falsifiability** | High | Can test: do generated files work? Is spec expressive enough? |

**Plausibility Verdict:** MARGINAL

### Assumptions to Verify
- [ ] A simple YAML spec can capture all FPF command semantics
- [ ] Generators are simple enough to maintain
- [ ] Overhead is justified by actual multi-platform usage
- [ ] Community would contribute generators for new platforms

### Required Evidence
- [ ] **Internal Test:** Draft spec for one command, generate Claude + Gemini outputs
  - **Performer:** Developer
- [ ] **Research:** Check if similar projects use this pattern (CLI framework comparisons)
  - **Performer:** AI Agent

## Falsification Criteria
- If spec cannot express platform-specific behaviors without becoming complex, this fails
- If generators require platform-specific logic that defeats the abstraction, this fails
- If overhead exceeds benefit for 3-4 platforms, this is overengineered

## Estimated Effort
High — 1-2 weeks for spec + generators + migration; ongoing schema maintenance

## Weakest Link
Complexity. This solves a problem that may not exist at current scale. 11 commands × 4 platforms might not justify a spec/generator architecture.
