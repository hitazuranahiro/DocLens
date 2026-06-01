# services/ingestion

Ingestion bounded context. Owns the upload intake and validation flow:

1. Validate the upload intent against MIME allow-list and size cap.
2. Dedupe `(ownerId, sha256)` against the Library to short-circuit
   re-uploads of the same file.
3. Issue a 5-minute presigned PUT URL.
4. On `/finalize`, verify the object landed and hand off to Library.

The Library and Ingestion contexts share the same database; that is by
design (per ADR 0006 schema-per-context, not database-per-context). They
talk to one another only through repository ports.

## Layout

```
ingestion/
├── domain/                 # Upload value object, errors, validation
└── app/                    # CreateUpload / FinalizeUpload use cases
```

Adapters live alongside domain when they land (M3 introduces only the
upload-intent flow; the orphan-sweep job lands together with M4 worker
infrastructure).
