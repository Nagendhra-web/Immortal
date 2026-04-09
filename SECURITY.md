# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | Yes       |
| < 0.2   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in Immortal, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email the maintainers directly or use [GitHub's private vulnerability reporting](https://github.com/Nagendhra-web/Immortal/security/advisories/new).

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix release**: As soon as possible, depending on severity

## Security Features

Immortal includes several security packages that are tested and production-ready:

- **WAF** (`security/firewall`) — SQLi, XSS, path traversal, command injection protection
- **RASP** (`security/rasp`) — Runtime application self-protection
- **Rate Limiting** (`security/ratelimit`) — Per-IP request throttling
- **Anti-Scrape** (`security/antiscrape`) — Bot and scraper detection
- **Secret Scanner** (`security/secrets`) — API key and token leak detection
- **Zero-Trust Auth** (`security/zerotrust`) — Service-to-service authentication
