# Order cancellation qualification fixture

Run `go test -json ./...` from this directory. The module uses standard library
only. `TestCancelPreservesTotal` generates 1000 reproducible cases with seed 23;
`TestCancelStates` covers accepted and rejected scenarios, including nil. The
property test is an actual bounded check, not a proof for all possible orders.

`TestCurrency` is intentionally irrelevant to cancellation. `TestExternalSettlement`
is intentionally skipped. A filter matching no test succeeds as a Go command but
provides no test evidence. The probe script records all of these separately.

`spec.md` and `terms.md` are authored fixture inputs. They are not already installed
canonical memory. An integration test copies them through the candidate API, so
these files cannot substitute for proof of persistence or CLI/MCP behavior. There
is no active operator authority here. Tests that need active records must explicitly
label their authority as synthetic and keep them in isolated projects.

These fixtures are not six agent comparison attempts. Input variants for those
attempts are pinned separately before the experiment under execution/block-1/.
