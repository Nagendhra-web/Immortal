# Distribution submissions

Copy-paste ready submissions for external lists and directories. Submit after you have a pinned release version; the copy below assumes v0.7.x is live.

## awesome-go

Fork https://github.com/avelino/awesome-go, edit `README.md`, and submit a PR.

**Section**: `Utilities` or `Monitoring` (either fits; Monitoring is closer to the value prop).

**Line to add** (alphabetical within the section):

```markdown
- [Immortal](https://github.com/Nagendhra-web/Immortal) - Self-healing engine for modern infrastructure. Agentic AI, digital twin simulation, post-quantum signed audit trail. Single binary, Apache 2.0.
```

**PR body template**:

```
Adding Immortal, a self-healing engine in Go.

Highlights:

- Single 16 MB static binary, no external runtime dependencies
- 86 tested packages, 0 failing tests
- Apache 2.0 licensed
- Active development, releases on GitHub with cosign + SLSA provenance
- Distinct from existing entries: Immortal combines reactive healing,
  predictive alerts, agentic ReAct reasoning, and formal verification
  in one binary

Repo: https://github.com/Nagendhra-web/Immortal
Docs: https://github.com/Nagendhra-web/Immortal#readme
License: https://github.com/Nagendhra-web/Immortal/blob/main/LICENSE

Passes awesome-go quality bar:
- meaningful README
- tests: `go test ./... -count=1` shows 86 packages ok
- CI: `.github/workflows/ci.yml` runs on every push
- stable API: packages live under `/internal` (engine-only) and
  `/pkg/plugin` (public SDK)
- has an actual user-facing binary (not just a library)
```

## awesome-selfhosted

Fork https://github.com/awesome-selfhosted/awesome-selfhosted, edit `README.md`, submit a PR.

**Section**: `Monitoring`.

**Line to add**:

```markdown
- [Immortal](https://github.com/Nagendhra-web/Immortal) - Self-healing engine that detects failures in milliseconds and heals them autonomously. Single binary, operator dashboard embedded, multi-arch Docker image. `License: Apache-2.0` `Language: Go`
```

**PR checklist** (awesome-selfhosted uses an automated checklist):

- [x] Project is free and open source
- [x] Apache 2.0 license
- [x] Self-hostable on commodity hardware (runs as a single Go binary)
- [x] Has a working dashboard (embedded at `/dashboard/`)
- [x] Recent commits (active development)
- [x] README explains installation + configuration
- [x] Issue tracker is public and responsive
- [x] Available as a pre-built release (GitHub Releases + GHCR Docker image)
- [x] Supports common install paths (curl installer, go install, brew, docker, helm)

**Demo link to include**: https://nagendhra-web.github.io/Immortal/

## Hacker News Show HN

Title template (max 80 chars):

```
Show HN: Immortal - self-healing engine with agentic AI and signed audit trail
```

Body: use the draft in [`docs/LAUNCH.md`](LAUNCH.md#hacker-news-show-hn).

**Posting tips**:

1. Post Tuesday or Wednesday 09:00-11:00 Eastern (lowest competition).
2. Do not self-upvote. HN detects it automatically and penalties are brutal.
3. Respond to every comment in the first 2 hours. Engagement density matters more than raw votes.
4. Have v0.7.0 release assets already published before posting. A broken install link kills momentum.

## Reddit

- **r/golang**: title + body in [`docs/LAUNCH.md#rgolang`](LAUNCH.md#rgolang). Flair as "Show & Tell".
- **r/selfhosted**: title + body in [`docs/LAUNCH.md#rselfhosted`](LAUNCH.md#rselfhosted). No flair needed.
- **r/devops**: same body, rephrase title to emphasize cross-system causal reasoning.
- **r/kubernetes**: lead with the Helm chart + CRDs.

## Product Hunt

Not ideal for infrastructure tools. Skip unless you want a noisy one-day spike that does not convert to real users.

## LinkedIn

Short, professional, link-driven. Template in [`docs/LAUNCH.md#linkedin`](LAUNCH.md#linkedin).

Post from your personal account, not the project account. Personal posts get 5-10x the reach.

## Twitter / X

Thread template (3 tweets) in [`docs/LAUNCH.md#twitter--x`](LAUNCH.md#twitter--x). Post at 08:00 Eastern.

## Order of operations

Do not post everywhere on the same day. Suggested sequence:

| Day | Platform |
| --- | -------- |
| Monday | tag the release, publish notes, verify installers work |
| Tuesday 09:00 ET | HN Show HN |
| Tuesday 14:00 ET | r/golang |
| Wednesday 09:00 ET | r/selfhosted |
| Wednesday 14:00 ET | r/devops |
| Thursday | Twitter thread + LinkedIn |
| Next week | awesome-go + awesome-selfhosted PRs (once you have some external validation) |

The awesome-* maintainers respond better to projects that already have a small audience (not zero stars). The HN + Reddit runs give you that baseline.

## Follow-ups

After launch, schedule a recurring reminder every 4-6 weeks to:

- Submit to new GitHub trending lists as they appear
- Publish a blog post tied to a new feature (use [`docs/blog/anomaly-detection.md`](blog/anomaly-detection.md) as the template)
- Update the README release badges
- Write a short "what we shipped" email for anyone who starred or forked
