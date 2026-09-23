# Synthetic Core publication for source-reader tests

These are authored parser fixtures, not FPF rule content.
The contents mention A.1 and A.10 without starting their bodies.

| Pattern | Intent |
| --- | --- |
| A.1 | Identify a fixture subject |

## A.1 - Identify the fixture subject

### A.1:1 - Problem frame
A maintainer needs to recover one exact subject before comparing its descriptions.

### A.1:4 - Solution
Name the subject and the distinguishing conditions. Pin B uses serial bravo.

```markdown
## A.99 - This is a fenced demonstration, not a pattern
### A.99:End
### A.1:End
```

### A.1:7 - Conformance checklist
- The subject remains distinct from its description.

### A.1:9 - Consequences
This fixture allows exact retrieval without selecting an engineering method.

### A.1:End

Inter-pattern prose is not part of either body.

## A.10 — Keep the fixture observation bounded

### A.10:1 - Problem frame
An observed test result is being used for one claim.

### A.10:4 - Solution
Preserve the exact target and tested inputs; a test locator is not a run.

### A.10:7 - Consequences
Missing observation leaves the attempted use unresolved.

### A.10:End
