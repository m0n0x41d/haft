---
format: haft/1
id: spec-20260923-00000001
kind: spec
title: Order cancellation preserves accounting amounts
status: proposed
origin: agent_proposal
about: domain:Billing.OrderCancellation
created_at: 2026-09-23T10:00:00Z
slug: order-cancel
terms: [Billing.Order, Billing.CancelableOrder]
receiving_use: Change direct domain cancellation and identify the right regression check.
claims:
  - id: total-preserved
    kind: law
    text: Successful cancellation of a new or paid order preserves its total and moves it to cancelled.
    implemented_by:
      - ref: sym:order.go::Order.Cancel
        covers: Direct domain transition only; HTTP and persistent storage are outside this fixture.
    checks:
      - ref: pbt:order_test.go::TestCancelPreservesTotal
        covers: New and paid states; 1000 generated signed integer totals, seed 23; no universal proof.
    examples:
      - id: paid-order
        given: A paid order with total 123
        when: Cancel is invoked
        then: Status is cancelled and total remains 123
  - id: cancelable
    kind: definition
    text: A cancelable order has status new or paid.
    terms: [Billing.CancelableOrder]
  - id: admission
    kind: guard
    text: Cancellation is admitted only for a non-nil cancelable order.
    refs: [cancelable]
    implemented_by:
      - ref: sym:order.go::Order.Cancel
        covers: The direct method entry condition and unchanged rejected orders.
    checks:
      - ref: test:order_test.go::TestCancelStates
        covers: Explicit new, paid, shipped, cancelled and nil scenarios; no HTTP or storage scope.
---
This is a synthetic qualification fixture, not an operator-approved production norm.
The Currency test deliberately does not check any claim in this section. Changing
the helper in policy.go may affect cancellation even when Order.Cancel is unchanged.
