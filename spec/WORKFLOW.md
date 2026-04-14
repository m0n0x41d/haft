# `.haft/workflow.md`

`workflow.md` is a small project policy file for how Haft should reason and execute inside this repo.

It is a hybrid document:

1. `Intent` is prose for human-readable direction.
2. `Defaults` is YAML for project-wide policy.
3. `Path Policies` is YAML for path-scoped overrides.
4. `Exceptions` is prose for edge cases that do not fit the defaults.

## Schema

### Intent

Plain markdown prose under `## Intent`.

### Defaults

The first fenced YAML block under `## Defaults` must match:

```yaml
mode: standard        # tactical | standard | deep
require_decision: true
require_verify: true
allow_autonomy: false
```

### Path Policies

The first fenced YAML block under `## Path Policies` is a list of path rules:

```yaml
- path: "internal/artifact/**"
  mode: standard
  require_decision: true
  require_verify: true
  allow_autonomy: false
```

Rules:

- `path` is required.
- `mode` is optional, but if present must be `tactical`, `standard`, or `deep`.
- boolean flags are optional overrides for matching paths.

### Exceptions

Plain markdown prose under `## Exceptions`.

## Runtime behavior

- `haft agent` prepends `Intent` + `Defaults` to the agent system prompt at startup.
- `haft serve` exposes the same prompt prefix through MCP initialize instructions.
- `haft init` creates an example `.haft/workflow.md` when the file does not exist.
