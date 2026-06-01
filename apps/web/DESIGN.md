# DESIGN.md — DocLens

## Overview

DocLens's design system bridges developer tooling and end-user polish. A
deep purple accent on near-black surfaces creates a premium,
security-focused feel. The library, reader, and upload flows are designed
to feel native to a knowledge-work tool while staying calm enough for
long reading sessions.

This document is the contract between the visual language and the
implementation. Tokens defined here map directly to CSS variables in
`src/app/globals.css` and to the Tailwind config. Anything not on this
page should not appear in production code without an update here first.

## Colors

### Primary Palette

| Token           | Hex       | Usage                 |
| --------------- | --------- | --------------------- |
| `color-brand`   | `#6C47FF` | Primary purple        |
| `color-bg`      | `#131316` | App background        |
| `color-surface` | `#1F0256` | Purple-tinted surface |
| `color-text`    | `#FFFFFF` | Primary text          |
| `color-light`   | `#F4F0FF` | Light mode surfaces   |

### Neutral Palette

| Token            | Hex       | Usage          |
| ---------------- | --------- | -------------- |
| `color-gray-950` | `#131316` | App background |
| `color-gray-800` | `#1E1E26` | Card surfaces  |
| `color-gray-600` | `#3D3D50` | Borders        |
| `color-gray-400` | `#7C7C99` | Muted text     |
| `color-gray-100` | `#E8E8F0` | Light surfaces |

### Semantic Colors

| Token           | Hex       | Usage                                |
| --------------- | --------- | ------------------------------------ |
| `color-success` | `#16A34A` | Ready documents, successful uploads  |
| `color-error`   | `#DC2626` | Failed extraction, invalid input     |
| `color-warning` | `#D97706` | Stale presigned URLs, retry prompts  |
| `color-info`    | `#2563EB` | Extraction in progress, queued state |

The brand purple is the only accent. Interactive elements (focus rings,
active links, primary buttons) use `color-brand`; everything else stays
neutral so document content remains the visual focus.

## Typography

| Role    | Family         | Size | Weight | Line Height |
| ------- | -------------- | ---- | ------ | ----------- |
| Display | Inter          | 48px | 700    | 1.1         |
| Heading | Inter          | 32px | 600    | 1.2         |
| Title   | Inter          | 20px | 600    | 1.3         |
| Body    | Inter          | 16px | 400    | 1.6         |
| Label   | Inter          | 13px | 500    | 1.4         |
| Caption | Inter          | 12px | 400    | 1.4         |
| Mono    | JetBrains Mono | 14px | 400    | 1.6         |

`Mono` is reserved for the Markdown pane and code blocks. Everything
else flows in Inter.

## Spacing

| Token      | Value | Usage                                 |
| ---------- | ----- | ------------------------------------- |
| `space-1`  | 4px   | Inline gaps, badge padding            |
| `space-2`  | 8px   | Field gaps, icon-to-text spacing      |
| `space-3`  | 12px  | List-item internal padding            |
| `space-4`  | 16px  | Form spacing, document-row padding    |
| `space-6`  | 24px  | Card padding, section gaps            |
| `space-8`  | 32px  | Modal padding, page header bottom gap |
| `space-12` | 48px  | Page margins on desktop               |

## Border Radius

| Token         | Value  | Usage                        |
| ------------- | ------ | ---------------------------- |
| `radius-sm`   | 6px    | Inputs, buttons, badge pills |
| `radius-md`   | 10px   | Cards, document rows         |
| `radius-lg`   | 16px   | Sign-in modal, dropzone      |
| `radius-full` | 9999px | Avatar, status dots          |

## Elevation

| Level       | Value                              | Usage            |
| ----------- | ---------------------------------- | ---------------- |
| `shadow-sm` | `0 1px 3px rgba(0,0,0,0.2)`        | Input fields     |
| `shadow-md` | `0 4px 16px rgba(108,71,255,0.15)` | Auth card, hover |
| `shadow-lg` | `0 12px 40px rgba(0,0,0,0.4)`      | Full modal       |

The medium shadow uses brand purple at low opacity — a subtle signature
that ties cards to the accent without being noisy.

## Motion

| Token         | Value          | Usage                         |
| ------------- | -------------- | ----------------------------- |
| `motion-fast` | 120ms ease-out | Hover, focus, micro-state     |
| `motion-base` | 200ms ease-out | Card lift, badge transition   |
| `motion-slow` | 320ms ease-out | Modal in/out, page transition |

Motion is a tool for status, not decoration. Documents that change state
(queued → extracting → ready) animate the badge swap; rows themselves
do not move.

## Components

### Sign-in card

- Centered card, white bg in light mode / `color-gray-800` in dark mode.
- 16px radius (`radius-lg`), 32px padding (`space-8`).
- Social providers (Google, GitHub) above email/password.
- "Secured by Clerk" footer badge — never hide this.
- `shadow-md` rest, `shadow-lg` on focus-within.

### User button (header)

- 32px avatar circle (`radius-full`).
- Click opens dropdown with name, email, sign out.
- Themed via Clerk's `appearance` prop to match this palette.

### Document row (library)

- Card with 16px radius (`radius-md`), 16px padding (`space-4`).
- Left: 64×80 page-1 thumbnail with the same radius. Emoji fallback
  (📄) for documents without a thumbnail.
- Right: title (Title), filename + size (Caption), status badge,
  metric chips (pages/words/confidence).
- Border `color-gray-600` rest → `color-gray-400` on hover. No
  background change — only the border telegraphs interactivity.
- Tabbing into the row applies a 2px brand-purple focus ring.

### Status badge

| Status       | Surface               | Text                   |
| ------------ | --------------------- | ---------------------- |
| `queued`     | `color-gray-800`      | `color-gray-400`       |
| `extracting` | `color-info` @ 12%    | `color-info` @ 100%    |
| `ready`      | `color-success` @ 12% | `color-success` @ 100% |
| `failed`     | `color-error` @ 12%   | `color-error` @ 100%   |
| `deleted`    | `color-gray-800`      | `color-gray-400`       |

Pill shape (`radius-full`), Label type, 4px vertical / 8px horizontal
padding. No icon at v0.1.

### Upload dropzone

- 2px dashed border, `radius-lg`. Brand purple on drag-over, error red
  on drag-reject.
- 180px minimum height. Centered headline + caption listing accepted
  formats and the size cap.
- Below the zone: queue list. Each queued file shows hashing →
  uploading → done/error states with the matching status colors.

### Document reader

- Two-pane responsive grid. Side-by-side at 1024px+; stacked below.
- Each pane is a card with `color-gray-800` background, `color-gray-600`
  border, 4px header strip naming the pane (`Original`, `Markdown`).
- PDF pane: react-pdf canvas centered with prev/next pagination.
- Markdown pane: `<pre>` rendering in Mono so copy yields plain
  Markdown source (Req 4.5).
- Independent scroll per pane (Req 4.3).

### Loading skeletons

- Pulse animation on `color-gray-800` rectangles. No spinners; the
  skeleton mirrors the final layout so the page never reflows when
  data arrives.

### Empty states

- Centered headline + caption + primary CTA.
- 12px dashed border, 48px vertical padding (`space-12`).
- Tone is helpful, never apologetic.

## Themes

DocLens ships dark by default. Light mode is supported via
`prefers-color-scheme` plus a manual toggle (M9). Token mappings
swap surfaces and text but **never the brand purple** — purple is
identity, not theme.

Light-mode overrides:

| Dark token       | Light token      |
| ---------------- | ---------------- |
| `color-bg`       | `color-light`    |
| `color-gray-800` | `color-gray-100` |
| `color-gray-600` | `color-gray-100` |
| `color-text`     | `color-gray-950` |

## Accessibility

- Every interactive element has a visible focus state using
  `color-brand` at 100% opacity, 2px offset.
- Body text contrast ≥ 7:1 against background; muted text ≥ 4.5:1.
- Status is never communicated by color alone — every badge carries a
  text label.
- Targets are at least 40×40px on mobile.

## Do's and Don'ts

### Do

- Use purple as the single accent color. Reach for it when something
  is interactive, focused, or branded.
- Treat dark backgrounds as the default. Light mode is opt-in.
- Keep document content as the visual centerpiece — chrome should
  recede.
- Preserve the "Secured by Clerk" badge in auth flows.
- Use the status badge pattern for every async state in the system
  (uploads, extractions, retries).

### Don't

- Don't introduce a second accent color. If you need a new color,
  reach for the semantic palette.
- Don't reduce auth form padding below 24px (`space-6`).
- Don't animate document rows on hover — only the border changes.
- Don't use brand purple for non-interactive surface fills; it loses
  meaning when overused.
- Don't hide the security branding or replace it with marketing copy.

## Implementation pointers

- CSS variables live in `apps/web/src/app/globals.css`.
- Tailwind theme extends in `apps/web/tailwind.config.ts`.
- Component primitives live in `apps/web/src/components/`.
- This document is the source of truth; the code follows.
