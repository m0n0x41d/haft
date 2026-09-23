---
format: haft/1
id: opt-20260923-00000001
kind: options
title: Candidate brokers
status: active
origin: agent_proposal
about: dir:internal/ingest
created_at: "2026-09-23T10:00:00Z"
question: Which broker meets the delivery requirement?
options:
  - id: kafka
    summary: Reuse the existing deployment
    weakest_link: Operational complexity
  - id: nats
    summary: Evaluate a smaller deployment
    weakest_link: Missing operational experience
comparison:
  characteristics: [delivery, operations]
  basis: Same workload and operator availability
  comparator: Retain tradeoffs without assigning numeric confidence
  non_dominated: [kafka, nats]
next_use: probe_again
probe: Observe delivery through broker restart
---
No option has been selected.
