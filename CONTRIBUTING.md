# Contributing to Haft

## Workflow

1. **Create an issue first** — Open an issue with the `proposal` label. Include:
   - Rationale — either FPF methodology alignment or UX improvement
   - Question or problem statement
   - Proposed solution

2. **Wait for agreement** — Do not create a PR until the proposal has been discussed and agreed upon in the issue.

3. **Check for existing work** — Before starting, verify no one else has picked up the same issue. Comment on the issue to claim it.

4. **Create PR to `dev` branch** — When ready, open a pull request targeting `dev`, not `main`. Link the original issue.

5. **Update the changelog** — Add your changes to `CHANGELOG.md` under the `[Unreleased]` section.

## Development Setup

```bash
# Clone and enter the project
git clone https://github.com/m0n0x41d/haft.git
cd haft

# Build the current binary
mkdir -p ~/.local/bin
go build -o ~/.local/bin/haft -trimpath .

# Run bounded local checks
task test
task lint

# Run the exact repository linter version
GOMAXPROCS=1 GOFLAGS=-p=1 \
  go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 \
  run --timeout=5m --concurrency=1
```

The Go module and binary are both named `haft`.

These commands are a bounded local preflight, not proof of CI or release
qualification. CI and release run their defined non-desktop Go package contour
under the race detector one package at a time, with a 180-minute ceiling for
each package, plus their workflow-specific checks. Consolidated P13 remains a
separate acceptance lane.

## Documentation expectations

- Current-facing docs should use `haft`, `haft_*`, and `/h-*` naming.
- Historical references to `quint-code`, `quint_*`, or `.quint/` should stay only where they document release history or migrations.
- Keep references to Anatoly Levenchuk and FPF intact.
- Do not forget that both **MCP tool mode** and **command-driven mode** are supported.

## Want to Help but No Proposal?

Check existing issues labeled `bug`, `documentation`, or `help wanted`. Leave a comment to express interest and wait for approval before starting work.

## Summary

```
Issue (proposal label) → Agreement → Claim issue → PR to dev → Update changelog
```
