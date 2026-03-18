# Example: Handling Payment Confirmations

Your checkout works. Stripe charges the card.
But three weeks later, finance finds $12,000 in "ghost payments" —
customers charged but never got access.

The webhook endpoint returned 200. Logs look clean.
What went wrong?

## Without Quint

Your AI suggests: *"Just add a webhook endpoint that activates the subscription"*

You ship it. It works in testing. Production looks fine.

Until it doesn't. Webhooks fail silently. Your endpoint timed out during a DB hiccup. Stripe retried, you processed it twice. A network blip ate three webhooks completely.

Now you're debugging production with no record of why you built it this way.

## With Quint

```bash
$ /q1-hypothesize "handle stripe payment confirmation"
```

AI generates competing approaches:

| # | Approach | Risk | Recovery |
|---|----------|------|----------|
| H1 | Webhook-only | Silent failures, no detection | None without manual audit |
| H2 | Webhook + sync processing | Timeout = lost event, retries = duplicates | Stripe retry (3 days) |
| H3 | Webhook → Queue + Polling backup | Complex, two code paths | Self-healing |

```bash
$ /q2-verify
```

AI checks each hypothesis for logical consistency:
- **H1 fails:** "No mechanism detects missed webhooks"
- **H2 partial:** "Idempotency key needed, still misses network failures"
- **H3 passes:** "Polling catches what webhooks miss, queue handles spikes"

```bash
$ /q3-validate
```

AI gathers evidence for surviving hypotheses:

```
H3 — Webhook → Queue + Polling backup

Evidence collected:
1. [Stripe docs] "Webhooks are not guaranteed" — CL2 (external docs)
2. [Internal test] Queue handles 10k events/sec — CL3 (same context)
3. [Case study] Company X lost $50k before adding polling — CL1 (different context)

R_eff = 0.72 (limited by CL1 case study)
```

```bash
$ /q4-audit
```

```
Bias check: No anchoring detected (H1 was initial suggestion, rejected)

Trust scores:
- H3: R_eff = 0.72 (WLNK: external case study)

Recommendation: H3 is viable. Consider refreshing case study
with internal metrics after 30 days in production.
```

```bash
$ /q5-decide
```

```
Decision: H3 — Async queue + 15-min polling reconciliation

Rationale:
- Webhook acknowledges immediately (200 in <100ms)
- Background job processes with idempotency check
- Polling job catches silent failures
- Accepted tradeoff: 15-min max delay for edge cases

Evidence: Stripe docs recommend polling backup.
Review trigger: If webhook success rate drops below 99%
```

## 3 weeks later

Finance asks: *"Why do we poll every 15 minutes? Can we remove it?"*

```bash
$ /q-query "payment confirmation architecture"
```

```
Decision: 2024-01-15 — H3 selected over webhook-only

Key evidence:
- Stripe admits webhook delivery "not guaranteed"
- Polling catches ~0.3% of transactions (measured)
- Removing polling = ~$400/month in silent failures

Recommendation: Keep polling. Document in runbook.
```
