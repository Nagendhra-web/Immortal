# Examples

Ready-to-use configuration snippets. Each file is a real example of something we use or have seen in the field.

## Intent contracts

Intents are declarative goals the engine must maintain. The engine decides what actions to take; you declare the invariants.

| File | What it does |
| ---- | ------------ |
| [`immortal.json`](immortal.json) | Minimal engine config (baseline settings, data dir, flags) |
| [`rules.json`](rules.json) | Legacy rule-based healing (still supported, use for simple cases) |
| [`protect-checkout.yaml`](protect-checkout.yaml) | `ProtectCheckout` contract: latency < 200 ms, errors < 0.5% on checkout + payments |
| [`never-drop-jobs.json`](never-drop-jobs.json) | `NeverDropJobs` contract: orders queue must never drop a message |
| [`cost-ceiling.yaml`](cost-ceiling.yaml) | `CostCeiling` contract: cap engine-driven spending at $12/hour |
| [`healing-rules.yaml`](healing-rules.yaml) | Five realistic healing rules: restart, cache clear, scale, circuit break, page human |

## Loading intents at runtime

```sh
# JSON
curl -X POST http://127.0.0.1:7777/api/v6/intent \
  -H 'Content-Type: application/json' \
  --data-binary @examples/never-drop-jobs.json

# YAML (requires yq)
yq -o=json . examples/protect-checkout.yaml | \
  curl -X POST http://127.0.0.1:7777/api/v6/intent \
       -H 'Content-Type: application/json' --data-binary @-

# Natural language (let the engine parse prose)
curl -X POST 'http://127.0.0.1:7777/api/v6/intent/compile?register=1' \
  -H 'Content-Type: application/json' \
  --data '{"text": "Protect checkout at all costs. Never drop jobs. Cap cost at 12 dollars per hour."}'
```

## Goal `kind` values

The numeric `kind` field in each Goal maps to:

| Value | Constant | Semantics |
| ----- | -------- | --------- |
| 0 | `LatencyUnder`    | latency must stay under `target` ms |
| 1 | `ErrorRateUnder`  | error rate must stay under `target` (0.0 - 1.0) |
| 2 | `AvailabilityOver`| availability must stay over `target` (0.0 - 1.0) |
| 3 | `JobsNoDrop`      | queue drop count must stay at `target` (typically 0) |
| 4 | `ProtectService`  | priority booster: conflicts resolved in favour of this service |
| 5 | `ProtectService` (alias) | same as 4 in the current build |
| 6 | `CostCap`         | aggregate $/hour must stay under `target` |
| 7 | `Saturation`      | resource saturation must stay under `target` (0.0 - 1.0) |

## Running the operator dashboard

Once the engine is running, the operator console renders each intent with live status:

```
open http://127.0.0.1:7777/dashboard/#/intent
```

The `/intent` view shows registered contracts, live goal evaluation, and ranked suggestions for any at-risk goal.

## Healing rules vs intents

Use **healing rules** (`healing-rules.yaml`) for:

- Simple "if X then Y" responses to known failure signatures
- Infrastructure actions that never change (restart, page, circuit break)

Use **intents** (YAML/JSON Goal declarations) for:

- Business-level invariants ("checkout must stay usable")
- Situations where the engine should pick among competing actions
- Scenarios where cost, priority, or blast-radius needs weighing

Both can run simultaneously; intents run on top of rules and override them when priorities conflict.
