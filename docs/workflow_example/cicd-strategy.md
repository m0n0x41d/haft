# Example: Choosing a CI/CD Strategy

Your legacy deployments work — SSH into server, git pull, build in place.
But every deployment is a prayer. No rollbacks, no consistency, no audit trail.

You're building a new service and want to do it right this time.

## The Problem

Current state:
- Git clone via SSH directly to EC2
- Build happens on the server
- No rollback mechanism
- "Works on my machine" is the deployment strategy

Requirements:
- Build/release idempotency
- Cost-effective (no Kubernetes)
- Must scale to other services later
- Private repos, AWS infrastructure

## With Quint

```bash
$ /q1-hypothesize "CICD strategy for new service - no k8s, cost-effective, idempotent"
```

AI generates competing approaches:

| # | Approach | Complexity | Cost | Rollback |
|---|----------|------------|------|----------|
| H1 | GitHub Actions + SSH deploy | Low | Free | Manual |
| H2 | Docker Swarm + ECR | Medium | ~$5/mo registry | Built-in |
| H3 | ECS Fargate | Medium-High | ~$30+/mo | Built-in |
| H4 | Kamal (DHH's tool) | Medium | Free | Built-in |

```bash
$ /q2-verify
```

AI checks constraints:
- **H1 fails:** "No idempotency — same problem as current setup"
- **H3 partial:** "Overkill for B2B fintech with <100 RPS"
- **H4 partial:** "Requires Ruby runtime on deploy machine"
- **H2 passes:** "Native Docker, no external dependencies, handles multi-service"

```bash
$ /q1-add "concerned about blast radius — all services on one Swarm node?"
```

AI generates sub-hypotheses:

| # | Approach | Blast Radius | Cost |
|---|----------|--------------|------|
| H2a | Single Swarm node, all services | High | Low (~$50/mo) |
| H2b | Swarm node per service | Low | Higher (~$150/mo) |
| H2c | Hybrid: Swarm for new, legacy stays | Medium | Medium |

```bash
$ /q3-validate
```

AI researches evidence:

```
H2 — Docker Swarm + ECR

Evidence collected:
1. [Docker docs] Swarm handles 1000s of containers per node — CL2
2. [AWS pricing] ECR ~$0.10/GB, t3.medium ~$30/mo — CL3 (our context)
3. [HN thread] "Swarm dead?" — maintainers confirm active, just feature-complete — CL1

R_eff = 0.68 (limited by HN thread uncertainty)
```

```bash
$ /q4-audit
```

```
Bias check: Initial preference was Kamal (H4) — rejected on Ruby dependency

Trust scores:
- H2b (Swarm per service): R_eff = 0.68

Cost analysis:
- Staging: Single Swarm node acceptable (blast radius OK for non-prod)
- Production: Swarm per service for isolation

Recommendation: H2b for prod, H2a for staging. Revisit if RPS > 1000.
```

```bash
$ /q5-decide
```

```
Decision: Docker Swarm + ECR

Architecture:
- Staging: Single t3.medium, all services in Swarm
- Production: t3.medium per service, Swarm mode
- Registry: AWS ECR (private, ~$5/mo)
- CI: GitHub Actions (free tier sufficient)

Rationale:
- Idempotent deploys via image tags
- Rolling updates with automatic rollback
- No Ruby/external dependencies
- Migration path for legacy services

Review trigger: If any service needs >1000 RPS or HA across AZs
```

When new developer asks: *"Why aren't we using Kubernetes? Everyone uses it."*

```bash
$ /q-query "CICD architecture decision"
```

```
Decision: 2024-02-15 — Docker Swarm selected over ECS/K8s

Key evidence:
- B2B fintech, <100 RPS typical load
- K8s operational overhead not justified
- Swarm provides 90% of benefits at 10% complexity
- Cost: ~$80/mo vs ~$300/mo for ECS

Recommendation: Revisit only if:
- Multi-AZ HA required
- RPS exceeds 1000 sustained
- Team grows beyond 5 engineers
```

**The decision survives team changes. No tribal knowledge required.**
