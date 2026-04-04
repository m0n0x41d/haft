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
git clone https://github.com/m0n0x41d/quint-code.git
cd quint-code

# Enable pre-commit hooks (mirrors CI pipeline exactly)
git config core.hooksPath .githooks

# Install golangci-lint (required for full lint checks)
# https://golangci-lint.run/welcome/install/
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Build the current binary
go build -o ~/.local/bin/haft -trimpath .

# Run tests
go test -race ./...
```

The GitHub repository path still uses the historical `quint-code` name. The current module and binary are `haft`.

The pre-commit hook runs the same checks as CI: `go mod tidy`, `golangci-lint`, `go test -race`, `go build`. If it fails locally, CI will fail too.

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
