# Source-reader fixtures

These short publications are authored test inputs, not copied FPF/DPF text and
not governing source content. Familiar IDs exercise the real heading grammar.
Their prose describes the intended distinctions without claiming to be an FPF
edition. All revisions created from them must identify the fixtures as fixtures.

`manifest.json` records literal byte hashes, lengths and expected successful
whole-pattern boundaries. It is a test oracle, not the runtime source format.
Do not normalize line endings before checking these values.

| Directory | Purpose |
| --- | --- |
| `pin-a` | Two Core patterns, one Engineering pattern and four navigation documents |
| `pin-b` | Independent changed source basis; Engineering publication uses CRLF |
| `malformed/duplicate-within` | Two complete occurrences of the same ID in one file |
| `malformed/duplicate-across` | The same ID in two separate publications |
| `malformed/missing-end` | Incomplete pattern before an independently complete neighbor |
| `malformed/mismatched-end` | End marker names another pattern |
| `malformed/duplicate-end` | Two matching End markers create ambiguous extraction |
| `capture/before`, `capture/after` | Different content with exactly equal byte length |

The Core publication includes PatternIDs in a contents table, an ordinary mention,
and a fenced Markdown example. The example contains a fake pattern and a fake
End for the surrounding real pattern. Neither is an extraction boundary.

For a deterministic capture race, a test filesystem/read hook should return the
`before` bytes on the first read and `after` bytes on verification; the filesystem
metadata can remain fixed. A stabilized retry may capture `after`. Alternating
the two indefinitely must end within the retry budget with an explicit source
instability result. A test must not depend on sleeping until a racing thread wins.

The detailed test design and proposed carrier integration are in the candidate
execution report `source-fixture-plan.md`. No source-reader implementation or
runtime qualification is implied by the presence of these fixtures.
