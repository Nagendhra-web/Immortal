# Security Policy

Immortal is a self-healing engine often deployed at the infrastructure boundary. We take vulnerability reports seriously and will respond within two business days.

## Supported Versions

| Version | Supported           |
| ------- | ------------------- |
| 0.5.x   | Yes                 |
| 0.4.x   | Critical fixes only |
| < 0.4   | No                  |

## Reporting a Vulnerability

**Do NOT open a public GitHub issue for security vulnerabilities.**

Preferred channel:

1. [GitHub Private Vulnerability Reporting](https://github.com/Nagendhra-web/Immortal/security/advisories/new). Encrypted, traceable, no public disclosure.

Alternative channel:

2. Email the maintainer directly at **nagendhra.madishetti24@gmail.com** with subject line prefix `[immortal-security]`.

## What to Include

- A clear description of the issue.
- Affected versions (commit SHA or release tag).
- Steps to reproduce, proof-of-concept code, or a minimal repro repo.
- Potential impact (what an attacker could achieve).
- Any suggested fix or mitigation.

## Our Process

1. **Acknowledge** within 48 hours.
2. **Triage and confirm** within 5 business days. We will keep you informed on progress.
3. **Fix** in a private branch. You will be invited to validate the patch before it lands.
4. **Coordinated disclosure**. We aim to publish a patch release and a public advisory within 30 days of confirmation. If a CVE is required, we will request one through GitHub.
5. **Credit** in the advisory and release notes unless you prefer to remain anonymous.

## Scope

In scope: anything inside this repository that ships as part of the Immortal binary, the SDKs under `sdk/`, and the Pages-hosted landing site.

Out of scope: third-party dependencies (report upstream), theoretical issues without a working exploit, vulnerabilities that require the attacker to already have root on the host.

## Safe Harbor

We will not pursue legal action against good-faith security research that follows this policy. If you are unsure whether your research is in scope, email first.

## Verifying a release

Starting in v0.7.x, every Immortal container image on GHCR ships with:

1. **Keyless Sigstore signature** via Fulcio (GitHub OIDC identity, no private key to manage).
2. **SLSA level 3 build provenance** attestation attached to the image manifest.
3. **SPDX SBOM** attached to the image manifest.

### Verify the signature

```sh
cosign verify ghcr.io/nagendhra-web/immortal:v0.7.0 \
  --certificate-identity-regexp 'https://github.com/Nagendhra-web/Immortal/\.github/workflows/release\.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

A valid signature confirms the image was built by our release workflow for the tag you expect. Mismatches mean someone tampered with the image or substituted a different one.

### Inspect the provenance

```sh
cosign download attestation ghcr.io/nagendhra-web/immortal:v0.7.0 \
  --predicate-type https://slsa.dev/provenance/v1
```

The provenance includes the source commit, workflow run ID, and builder identity. It is signed with the same Fulcio-issued cert.

### Inspect the SBOM

```sh
cosign download sbom ghcr.io/nagendhra-web/immortal:v0.7.0
```

Returns a SPDX-JSON document listing every Go module and OS package in the image.

### Who can verify

Anyone. Keyless signatures do not require a shared secret. The only trust root is Sigstore's public Fulcio CA + the GitHub OIDC issuer. Airgap-friendly verification is also possible if you pin the Sigstore TUF root ahead of time.

## Security Features in Immortal Itself

These packages ship with the binary and are covered by the policy above:

- **WAF** (`internal/security/firewall`). SQLi, XSS, path traversal, command injection.
- **RASP** (`internal/security/rasp`). Runtime protection against dangerous operations.
- **Rate limiter** (`internal/security/ratelimit`). Per-IP throttling.
- **Anti-scrape** (`internal/security/antiscrape`). Bot and scraper detection.
- **Secret scanner** (`internal/security/secrets`). Find leaked keys, tokens, passwords.
- **Zero-trust auth** (`internal/security/zerotrust`). Service-to-service tokens with expiry.
- **Post-quantum audit chain** (`internal/pqaudit`). Hash-chained, Merkle-rooted, signer-pluggable.
