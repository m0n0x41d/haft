---
format: haft/1
id: dec-20260923-00000001
kind: decision
title: Use existing Kafka operations
status: proposed
origin: agent_proposal
about: dir:internal/ingest
created_at: "2026-09-23T10:00:00Z"
object: Ingestion broker
question: Which broker supports the required delivery contract?
disposition: choose_now
chosen: kafka
why: The team already operates it under the required delivery contract
options:
  - id: nats
    verdict: rejected
    reason: The team has no operational evidence for it
constrains: [dir:internal/ingest]
---
This is a proposal, not authorization.
