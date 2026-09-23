---
format: haft/1
id: spec-20260924-00000001
kind: spec
title: Reservation conserves stock
status: proposed
origin: agent_proposal
about: domain:Inventory.Reservation
created_at: 2026-09-24T00:00:00Z
slug: stock-reservation
receiving_use: Verify a second independent project through the same claim and check API.
claims:
  - id: conservation
    kind: law
    text: A successful reservation transfers exactly the requested quantity from available to reserved stock.
    implemented_by:
      - ref: sym:reservation.go::Stock.Reserve
        covers: In-memory stock quantities only; no concurrency or database contract.
    checks:
      - ref: pbt:reservation_test.go::TestReservationConservesStock
        covers: 1000 generated uint32 quantities and stock levels with seed 24; finite check only.
    examples:
      - id: reserve-two
        given: Five available and two reserved units.
        when: Two more units are reserved.
        then: Three available and four reserved units remain.
  - id: admission
    kind: guard
    text: Nil stock, zero quantity, insufficient stock and reserved-counter overflow are rejected without mutation.
    implemented_by:
      - ref: sym:reservation.go::Stock.Reserve
        covers: Direct method invocation and its admitted helper.
    checks:
      - ref: test:reservation_test.go::TestReservationRejectsWithoutMutation
        covers: Zero, insufficient quantity, uint64 overflow and nil stock scenarios.
---
Synthetic second-project qualification material; no production approval is claimed.
The stock fixture has a distinct module, symbol, property and subject from orders.
