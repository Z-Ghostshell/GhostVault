# Security

Thank you for helping keep GhostVault and its users safe.

## Supported versions

Security fixes are applied to the **latest commit on the default branch** of this repository. There are no numbered LTS releases yet; pin a commit hash or tag for production and watch this repo for updates.

## Reporting a vulnerability

**Please do not open a public GitHub issue that includes exploit details.**

If this repository has **[private vulnerability reporting](https://docs.github.com/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)** enabled on GitHub, use that. Otherwise, contact the maintainers through a private channel (for example an email published on the organization or maintainer profile).

Include, when you can:

- A short description of the issue and its impact
- Steps to reproduce (or a proof-of-concept)
- Affected component (e.g. `gvsvd`, `gvmcp`, dashboard, edge nginx)

Maintainers should acknowledge within a reasonable window and coordinate a fix and disclosure timeline. Credit in release notes is offered if you want it.

## Operational hardening (high level)

- **Never expose `gvsvd` or `gvmcp` to the public internet** without TLS, network policy, and (for browser or multi-tenant use) an additional authentication layer in front of the API.
- **Treat bearer tokens and MCP path tokens as secrets.** Rotate them after compromise or suspected leak.
- **Keep `GV_DEBUG_AUTH_FULL` / `GHOSTVAULT_DEBUG_AUTH_FULL` off** in any environment where logs are aggregated or retained.
- **Use strong Postgres credentials and TLS** to the database in real deployments; the bundled `docker-compose.yml` is for local development only.
- See [docs/UNLOCK-AND-BEARER.md](docs/UNLOCK-AND-BEARER.md) and [docs/future/THREAT-MODEL.md](docs/future/THREAT-MODEL.md) for the credential and threat model.
