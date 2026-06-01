# ADR 0008: Auth via Clerk for v0.1, behind a port

## Status

Accepted

## Date

2026-06-01

## Context

Authentication is high-risk to build, low-differentiation, and a frequent source of CVEs. We need:

- Email/password and social sign-in (Google, GitHub) for v0.1.
- Sessions that work cleanly with Next.js App Router and Server Components.
- A path to self-hostable auth for users who reject SaaS dependencies.

## Decision

For the hosted DocLens, use **Clerk**. It has first-class Next.js App Router support, generous free tier, and reasonable enterprise pricing. We accept the SaaS dependency for v0.1 because it lets us ship the product features instead of writing identity plumbing.

We do not couple to Clerk. The API defines an `Authenticator` port:

```go
type Authenticator interface {
    Verify(ctx context.Context, token string) (Identity, error)
}

type Identity struct {
    UserID string
    Email  string
    Claims map[string]any
}
```

Adapters: `auth/clerk` for hosted, `auth/supabase` and `auth/local` planned for self-host. Selection is config-driven.

Frontend talks to Clerk directly for sign-in flows. Frontend sends `Authorization: Bearer <jwt>`. The API verifies via the configured authenticator.

## Alternatives Considered

- **Build it in-house with Lucia / Auth.js.** Lucia is excellent, but session, MFA, password reset, abuse protection, audit logging, and SOC2 readiness add up to weeks. Reconsider for self-host edition.
- **Supabase Auth.** Reasonable. Pinned to Supabase; we may add it as a second adapter.
- **Auth0.** More expensive, less Next.js-native than Clerk in 2026.
- **Keycloak.** Best self-host option, but operational overhead is high for v0.1.

## Consequences

**Positive**

- Sign-in works on day one with social providers and email magic links.
- The port pattern keeps Clerk replaceable.
- Frontend uses the official Clerk components.

**Negative**

- A vendor in the auth path for the hosted version.
- Tests for the API need a fake `Authenticator` (we provide one in `services/shared/auth/fake`).

**Neutral**

- Authorization (who can read which document) is application-owned. Clerk only authenticates.
