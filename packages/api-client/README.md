# @doclens/api-client

Type-safe DocLens API client generated from `apps/api/openapi.yaml`.

## Usage

```ts
import { createDocLensClient } from "@doclens/api-client";

const client = createDocLensClient({
  baseUrl: process.env.PUBLIC_API_URL!,
  tokenProvider: () => getClerkToken(),
});

const { data, error } = await client.GET("/v1/health");
```

## Regenerating

```bash
pnpm --filter @doclens/api-client gen
```

This runs `openapi-typescript` against the source spec at `apps/api/openapi.yaml` and writes `src/generated/schema.ts`. Regenerate whenever the API spec changes; CI fails if the generated file drifts.
