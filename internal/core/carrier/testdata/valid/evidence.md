---
format: haft/1
id: ev-20260923-00000001
kind: evidence
title: Cancellation property result
status: active
origin: agent_proposal
about: domain:Billing.OrderCancellation
created_at: "2026-09-23T10:00:00Z"
claim: The total remained unchanged in the exercised cases
observed_at: "2026-09-23T09:59:00Z"
method: Property test
source: reports/cancel-property.txt
basis:
  kind: code
  ref: build:cancel-fixture-001
  conditions: Go property fixture, seed 42, new and paid orders
uses:
  - id: u1
    target: spec-20260923-00000001@sha256:0000000000000000000000000000000000000000000000000000000000000000#L1
    check: pbt:internal/order/cancel_test.go::TestCancelPreservesTotal
    polarity: supports
    scope: Domain operation for new and paid orders
    disposition: pass
    receiving_use: Assess this candidate implementation
---
The report is the result source; schema validation does not execute a test.
