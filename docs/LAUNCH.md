# Launch copy

Reusable announcement templates for `v0.6.0`. Pick the closest channel and tailor.

---

## Hacker News (Show HN)

**Title** (keep under 80 chars):

> Show HN: Immortal - self-healing engine with agentic AI and signed audit trail

**Body**:

> Immortal is an open-source self-healing engine for modern infrastructure. It does the obvious thing (detect failures, restart services) but adds the parts that are usually manual: predicting incidents before they fire, simulating fixes against a digital twin before touching prod, and proving every action with a post-quantum signed audit chain.
>
> What's in it:
>
> - Reactive + predictive + agentic ReAct healing loop
> - Digital twin that rejects plans that worsen the predicted health score
> - PCMCI causal root-cause analysis (not just correlation)
> - LTL/CTL model checker with counterexample generation
> - Federated anomaly learning across a fleet; raw metrics never leave the node
> - Post-quantum audit chain (hash-chained, Merkle-rooted, signer-pluggable)
> - 12-view operator dashboard shipped inside the single binary
>
> 79 Go packages, 0 failing tests, Apache 2.0. 16 MB single binary. Runs on a Raspberry Pi or a 128-core bare-metal node.
>
> Install: `curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash`
>
> Repo: https://github.com/Nagendhra-web/Immortal
> Landing + live demo: https://nagendhra-web.github.io/Immortal/
>
> Curious what folks here think about agentic healing for production infrastructure. Questions, skepticism, counter-examples all welcome.

---

## r/golang

**Title**: `Immortal v0.6.0 - self-healing engine in one Go binary (agentic + digital twin + PQ audit)`

**Body**:

> Built in Go. 79 packages. Single 16 MB static binary. Apache 2.0.
>
> Features I think this community will care about:
>
> - Uses `embed.FS` for the full operator dashboard (12 views, zero JS deps, pure vanilla HTML/CSS/JS) - no webpack, no Vite, no node_modules in prod.
> - Runs the whole healing orchestrator (reactive + predictive + agentic ReAct) + digital twin + PCMCI causal inference + TLA+-style formal verifier + post-quantum audit chain in one process.
> - Every package works standalone: `go get github.com/Nagendhra-web/Immortal` and import just `internal/dna` for anomaly detection or just `internal/twin` for simulation.
> - Tests cover end-to-end scenarios: cascading failure -> twin rejects bad plan, attacker mutates audit log -> Merkle root exposes it, federated fleet resists a malicious outlier via robust trim.
>
> Repo: https://github.com/Nagendhra-web/Immortal
> Install: `go install github.com/Nagendhra-web/Immortal/cmd/immortal@latest`
> Landing: https://nagendhra-web.github.io/Immortal/
>
> Interested in feedback on the plugin SDK design (`pkg/plugin`) and the twin simulation model.

---

## r/selfhosted

**Title**: `Self-healing engine that actually runs on your hardware (air-gap friendly, no phone-home)`

**Body**:

> If you self-host anything that must not die (Plex, Nextcloud, a home lab, your side project), Immortal watches it and heals it when it breaks. Runs locally, no external dependencies, Apache 2.0.
>
> What it can heal:
>
> - HTTP endpoints returning 5xx -> restart the service
> - Log patterns matching crash signatures -> trigger recovery playbook
> - CPU / memory / disk anomalies vs learned baseline -> scale / clear cache / page someone
> - Cascading failures (e.g. Postgres dies -> API 500s -> queue backs up) -> trace the chain and fix at the root
>
> What it does not do: phone home, collect telemetry, require a cloud account, or cost money.
>
> Single binary, runs on Raspberry Pi, ships a built-in web dashboard at `:7777/dashboard/`.
>
> `curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash`
>
> Repo: https://github.com/Nagendhra-web/Immortal

---

## r/devops / r/sre

**Title**: `We built a self-healing engine that predicts incidents, simulates the fix, and signs every action (open source)`

**Body**:

Lead with the why:
- Every layer of the stack is automated. Deploys, tests, infra. Recovery is still manual.
- A human is still in the loop at 3 AM, reading stack traces, guessing at root causes.

Lead with the how:
- Predictive healing via linear regression and anomaly detection catches breaches early.
- Agentic ReAct loop plans a fix, simulates it in the twin, verifies the post-condition.
- Post-quantum signed audit trail means every action has a receipt nobody can tamper with.

End with the ask:
- Would love feedback from anyone running SRE at scale. Honest skepticism welcome.

Repo + quickstart: https://github.com/Nagendhra-web/Immortal

---

## awesome-go PR

**Line to add under "Monitoring" or "Self-healing"**:

```
- [Immortal](https://github.com/Nagendhra-web/Immortal) - Self-healing engine for modern infrastructure. Agentic AI, digital twin simulation, post-quantum signed audit trail. Single binary, Apache 2.0.
```

PR description template:

```
Adding Immortal v0.6.0, a self-healing engine in Go.

- Single 16 MB static binary, no external runtime deps
- 79 tested packages
- Apache 2.0 licensed
- Active development, full test suite green in CI
- Not a port or wrapper of another tool

Repo follows awesome-go style guidelines: meaningful README, tests, CI, license, active maintenance.
```

---

## Twitter / X

**Tweet 1 (hook)**:

> Every layer of your stack is automated. Deploys, tests, infra.
>
> Recovery is still manual. A human is still in the loop at 3 AM.
>
> Immortal closes that loop.
>
> Open source, single Go binary, Apache 2.0. v0.6.0 out now.

**Reply (features)**:

> What's inside:
> - Agentic ReAct healing loop
> - Digital twin that simulates the fix first
> - PCMCI causal root-cause
> - Post-quantum signed audit trail
> - 12-view operator dashboard in the binary
>
> https://github.com/Nagendhra-web/Immortal

**Reply (install)**:

> `curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash`
>
> Landing page: https://nagendhra-web.github.io/Immortal/

---

## LinkedIn

Professional tone, lead with the problem:

> I have been building Immortal for the past few months. It is the first open-source self-healing engine that combines agentic AI with formal verification and a post-quantum audit trail.
>
> Why it exists: deployments, tests, and infra are all automated. Recovery is not. A human still reads stack traces at 3 AM.
>
> v0.6.0 just shipped:
>
> - Agentic ReAct healing loop for novel failures
> - Digital twin simulation before any plan touches prod
> - Post-quantum signed audit chain (Merkle-rooted, pluggable signer)
> - 12-view operator dashboard, single binary
>
> Apache 2.0. Runs anywhere Go runs.
>
> Repo: https://github.com/Nagendhra-web/Immortal
> Landing: https://nagendhra-web.github.io/Immortal/
>
> Feedback from SRE and platform engineers very welcome.

---

## Tips

- **Post Tuesday or Wednesday between 09:00 and 11:00 your target timezone.** HN frontpage churn is lowest then.
- **One channel at a time.** Wait at least 4 hours between posts so you can respond to comments.
- **Respond to every comment in the first 2 hours.** Upvotes correlate with engagement density.
- **Do not self-upvote or ring-vote.** Detection is automatic and penalties are brutal.
- **Have v0.6.0 release assets published before posting.** A broken install link kills momentum.
