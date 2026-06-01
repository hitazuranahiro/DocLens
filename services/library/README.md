# services/library

Library bounded context. Owns the canonical document record, status
lifecycle, and (M5+) the user-facing read model.

## Layout

```
library/
├── domain/                 # Document aggregate, status enum, errors
├── app/                    # Use cases (only orchestration, no SQL)
└── adapters/
    └── postgres/           # sqlc-generated + repo wrapper
```

The HTTP layer that exposes Library use cases lives in `apps/api/internal/handlers/`.
The Library module deliberately does not depend on chi or HTTP types so it
can be reused from the extraction worker (M4) and the SSE pusher (M6).
