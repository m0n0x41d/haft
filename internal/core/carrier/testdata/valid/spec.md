---
format: haft/1
id: spec-20260923-00000001
kind: spec
title: Cancel preserves total
status: active
origin: operator_request
operator_confirmed: true
about: domain:Billing.OrderCancellation
created_at: "2026-09-23T10:00:00Z"
slug: order-cancel
receiving_use: Verify cancellation before modifying the domain operation
terms: [Billing.Order]
constrains: [file:internal/order/cancel.go]
claims:
  - id: L1
    kind: law
    text: Cancel preserves total
    checks:
      - ref: pbt:internal/order/cancel_test.go::TestCancelPreservesTotal
        covers: Domain transition for new and paid orders
    implemented_by:
      - ref: sym:internal/order/cancel.go::Order.Cancel
        covers: Domain operation; excludes HTTP validation
    examples:
      - id: paid-order
        given: A paid order
        when: Cancel runs
        then: The total is unchanged
    custom_claim: {reviewer: Ivan, values: [1, 2]}
  - id: L2
    kind: definition
    text: Cancellable means new or paid
  - id: A1
    kind: guard
    text: Cancellation requires a cancellable order
    refs: [L2]
    unchecked: HTTP validation has not been exercised
extension:
  owner: Billing
  nested: [one, {two: three}]
---
# Cancellation

Preserve this prose and its **formatting**.
