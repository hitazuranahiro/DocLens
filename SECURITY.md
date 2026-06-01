# Security Policy

## Supported Versions

DocLens is pre-1.0. Security fixes target the latest `main` and the latest tagged release.

| Version       | Supported          |
| ------------- | ------------------ |
| `main`        | :white_check_mark: |
| Latest release | :white_check_mark: |
| Older         | :x:                |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security problems.

Use GitHub's private vulnerability reporting:
[Open a security advisory](https://github.com/hitazuranahiro/DocLens/security/advisories/new).

What to include:

- A description of the vulnerability and its impact.
- Steps to reproduce, ideally a minimal proof of concept.
- Affected component (`apps/api`, `apps/web`, `services/<context>`, infra, etc.) and commit SHA.
- Your suggested severity (low / medium / high / critical) and any known mitigations.

We aim to:

- Acknowledge the report within 3 business days.
- Provide an initial assessment within 7 business days.
- Coordinate a fix and disclosure timeline based on severity.

## Scope

In scope:

- Code in this repository (`apps/`, `services/`, `packages/`, `infra/`).
- Dependency vulnerabilities that materially affect DocLens.
- CI/CD workflows under `.github/workflows/`.

Out of scope:

- Issues that require physical access or compromised developer machines.
- Vulnerabilities in third-party services we integrate with — please report those upstream.

## Recognition

Reporters who follow this policy will be credited in release notes (with their consent) once a fix is shipped.
