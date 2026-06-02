# DocLens

> Turn Documents Into Knowledge.

DocLens is an open-source document intelligence platform that transforms PDFs, research papers, reports, manuals, and other documents into searchable, structured, and AI-ready knowledge.

Instead of simply extracting text, DocLens helps users understand, explore, and interact with information through AI-powered summaries, knowledge graphs, document search, and intelligent analysis.

![DocLens Banner](/Brand-Assets/banner.png)

---

## Quickstart

DocLens runs locally as a single \`docker compose\` stack: Postgres, Redis, MinIO, the Go API, the extraction worker, and the Next.js web client. The whole loop — sign in, upload a PDF, watch it extract, read the Markdown, search across documents, delete — runs offline against the local stack with no third-party services.

### Prerequisites

- Docker Desktop (or \`colima\` on macOS) with the \`docker compose\` plugin
- Node 20+ and pnpm 9+
- Go 1.23+
- A few minutes; first \`make dev\` pulls images and runs migrations

### Bring it up

```bash
git clone https://github.com/tomeku/doclens.git
cd doclens
make bootstrap         # installs JS deps + syncs the Go workspace
make dev               # starts Postgres, Redis, MinIO, api, worker, web
```

You should see:

- API healthy at <http://localhost:8080/v1/health>
- Web at <http://localhost:3000>
- MinIO console at <http://localhost:9001> (user/pass: \`doclens\`/\`doclens\`)

The API ships with a local-auth provider for development (\`AUTH_PROVIDER=local\`); no Clerk account is required to upload documents and exercise the full flow. To enable Clerk for browser sign-in, set \`NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY\` and \`CLERK_SECRET_KEY\` in \`apps/web/.env\` and \`AUTH_PROVIDER=clerk\` plus the Clerk envs in \`apps/api/.env\`.

### Common tasks

```bash
make test              # Go test ./... + pnpm vitest run
make lint              # golangci-lint + eslint
make gen               # regenerate OpenAPI client + server stubs
make migrate           # apply Postgres migrations against the running compose stack
make down              # stop everything
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full development workflow.

---

## Why DocLens?

Modern document extraction tools focus on converting files into text.

DocLens focuses on helping humans understand information.

Upload a document and instantly:

- 📄 Extract text, tables, images, and metadata
- 🧠 Generate AI-powered summaries
- 🔍 Search documents intelligently
- 🌐 Visualize relationships using knowledge graphs
- 🛡️ Verify extraction confidence
- ⚡ Prepare content for AI and RAG workflows

---

## Features

### Smart Extraction

Extract structured content from:

- PDFs
- Research Papers
- Reports
- Manuals
- Documentation
- Office Documents

---

### AI Summaries

Generate concise summaries and key insights.

```text
TL;DR
Key Findings
Important Concepts
Actionable Insights
```

---

### Knowledge Graphs

Visualize connections between:

- Topics
- Concepts
- Entities
- References

---

### Verification & Confidence

Know how reliable extracted information is.

```text
Confidence Score: 92%
Tables Found: 12
Topics Extracted: 23
```

---

### Search & Discovery

Find information instantly across large documents.

```text
"What does this document say about neural networks?"
```

---

## Use Cases

### Students

Study smarter with AI-generated summaries.

### Researchers

Extract findings and discover connections across papers.

### Developers

Prepare documents for RAG pipelines and AI applications.

### Businesses

Analyze reports, contracts, and manuals faster.

---

## Vision

We believe documents should be:

- Searchable
- Understandable
- Verifiable
- AI-ready

DocLens aims to become the open platform for document intelligence.

---

## Architecture

```text
Document
    ↓
Extraction Engine
    ↓
Document Intelligence Layer
    ↓
AI Processing
    ↓
Knowledge Graph
    ↓
Human Interface Layer
```

---

## Roadmap

### v0.1

- Document Upload
- Text Extraction
- Markdown Output
- Search

### v0.2

- AI Summaries
- Metadata Extraction
- Table Detection

### v0.3

- Knowledge Graphs
- Confidence Scoring
- Verification Layer

### v0.4

- RAG Export
- Embeddings
- AI Agents

### v1.0

- Full Document Intelligence Platform

---

## Tech Stack

### Frontend

- Next.js
- React
- TypeScript
- Tailwind CSS
- shadcn/ui

### Backend

- Go
- PostgreSQL
- Redis

### AI

- OpenAI
- Anthropic
- AWS Bedrock

### Infrastructure

- Docker
- Kubernetes
- Cloudflare

---

## Contributing

Contributions are welcome.

Whether you're a developer, designer, researcher, or student, we'd love your help building the future of document intelligence.

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Open a Pull Request

---

## Community

Website: https://doclens.org

GitHub Discussions: Coming Soon

Discord: Coming Soon

---

## License

MIT License

---

## Built with Love

Built with ❤️ by Tomeku.

https://tomeku.com
