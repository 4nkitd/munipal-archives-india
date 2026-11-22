# Design Specification — Nagar Archive (Municipal Archives India)

## Visual Direction

**Indian Civic Archive & Search-First Ledger**: inspired by Survey of India cadastral sheets, official gazette indices, and high-utility command search tools. Clean, authoritative, readable, and fast — never a cluttered government portal and never a generic drop-shadow SaaS card grid.

## Color Tokens

- `--canvas`: `#F7F8FA` — Archival bond paper background (cool-neutral alabaster, NOT warm cream)
- `--surface`: `#FFFFFF` — Clean sheet surface for results, reader drawer, and tables
- `--surface-subtle`: `#EEF1F5` — Soft ledger tint for filter strips, metadata rows, and code/citation pills
- `--ink`: `#132030` — Deep Prussian civic ink for headings and primary text
- `--ink-muted`: `#4E5D6C` — Graphite ledger ink for metadata, captions, and secondary copy
- `--accent`: `#1B4D7E` — Survey Blueprint Cobalt for focus borders, primary buttons, active tabs, and citations
- `--accent-hover`: `#143B61` — Deep cobalt hover state
- `--accent-soft`: `#E8F0F8` — Soft cobalt wash for active filters and citation highlights
- `--stamp`: `#9A5B13` — Official Revenue Ochre/Brass strictly for verified official badges, statutory deadlines, and active alerts
- `--stamp-soft`: `#FDF4E7` — Pale ochre tint for deadline/alert chips
- `--rule`: `#D6DCE5` — Crisp 1px structural ledger hairline
- `--rule-strong`: `#B7C2D0` — Stronger rule for inputs and primary dividers

## Implementation Stack (100% Pure Go)

- Built strictly in **Go** (`go1.27+`) with zero Node.js, npm, Vite, React, or external frontend build step.
- Server-rendered templates in `internal/web/templates/*.html` + static assets in `internal/web/static/` (`styles.css`, `app.js`), embedded into the Go binary via `//go:embed` in `internal/web/embed.go`.
- Markdown rendering via `github.com/yuin/goldmark` (with GFM tables enabled) directly in Go.
- Progressive enhancement: all primary views (`/`, `/search`, `/doc/{municipality}/{id}`, `/wards`, `/index`) render complete semantic HTML directly from Go AND enhance smoothly in-browser with minimal vanilla JS (`app.js`) for `⌘K` search focus, live `/api/suggest` typeahead, instant `/api/ask` updates, slide-over document reader drawer, and instant ward/officer filtering.

## Typography

Load from Google Fonts in `internal/web/templates/layout.html`:
- Display & Editorial Headings: `Newsreader` (weights 400, 500, 600; optical sizing)
- UI, Body & Tables: `IBM Plex Sans` (weights 400, 500, 600; use `font-variant-numeric: tabular-nums` on numbers, phones, dates, ward IDs, and rupee amounts)
- Identifiers Only: `IBM Plex Mono` (weights 400, 500; strictly for document IDs, tender reference IDs, and property IDs)

### Typographic Rules
- Sentence case everywhere (headings, nav tabs, facet headers, buttons, chips). Never ALL-CAPS tracked eyebrows.
- Keep prose line length <= 76ch (`max-w-[74ch]`), serif body/summary line-height 1.6, sans body 1.55.
- Never highlight a single word in a headline with italic or accent color.
- Never use middle-dot chains (`A · B · C`) or append `→` arrows to buttons/links. Use structured metadata rows or spaced pills instead.

## Layout Architecture

### 1. Top Navigation Bar (`56px`, sticky, `#FFFFFF`, `1px solid #D6DCE5` bottom rule)
- Left: Archive identity (`Nagar Archive` in `Newsreader` 600) + **Municipality Switcher** pill (`Municipal Corporation Manesar` with Haryana indicator; dropdown displays live indexed corporations plus extensible multi-city schema status).
- Right: Three concise workspace tabs (`Search`, `Wards & officers`, `Archive index`). Active tab uses a crisp 2px bottom border in `--accent` (`#1B4D7E`), never a filled colored pill.

### 2. Search-First Workspace (Default View)
- **Empty query state (Search Desk)**: Asymmetric two-column layout (`max-w-[1120px] mx-auto px-6 py-12`, grid `1fr` on mobile, `minmax(0, 1.75fr) minmax(280px, 1fr)` on desktop).
  - Left column: Clear headline ("Search the civic record of Manesar"), one concise sentence explaining parsed text archives + hybrid vector search, prominent search bar with `⌘K` focus shortcut, live typeahead suggestion dropdown, inline search mode switcher (`Hybrid`, `Semantic`, `Exact`), and clickable **Citizen Query Prompts** (real questions citizens ask).
  - Right column: Hairline-ruled **Municipal Ledger** displaying live telemetry from `/api/stats` (indexed records, vector chunks, 20 wards, 30+4 villages, 63 sectors, 42+ officers) and direct emergency/helpdesk numbers (`1800-180-1817`, `0124-2012232`).
- **Active search state**: Search bar docks cleanly into a compact top strip (`bg-white border-b border-[--rule]`). Below: two-column workspace (`248px` left Facet Filter Rail + `1fr` Results Stream).
  - **Prompted Archive Brief** (top of Results Stream — the hero interactive element): White sheet with `3px solid #1B4D7E` left accent rule and `1px solid #D6DCE5` outer rule. Shows synthesized answer summary, structured **Extracted Civic Facts** (responsible officer + clickable phone/email, statutory deadline, official portal button, matching wards/villages/sectors), inline numbered citations `[1]` `[2]` that open the exact archival record in the Document Reader Drawer, and clickable follow-up search prompts.
  - **Ranked Archive Results**: Full-width ledger rows separated by `1px solid #D6DCE5` (NOT drop-shadow cards). Each row shows category tag, date, ward tags, clickable title in `Newsreader`, highlighted matching snippet, source portal label, and a compact hybrid retrieval breakdown (`Hybrid score`, `Vector` vs `Keyword` match indicator).

### 3. Document Reader Slide-Over Drawer
- Opens over the right side (`max-w-[720px] w-full`) with backdrop and `Escape` to close, preserving the active search query underneath.
- Displays:
  - Sticky drawer header with title, document ID (`IBM Plex Mono`), "Copy text" button, and "Original source" external link.
  - Structured **Provenance Ledger** table (Category, Document type, Date, Department, Wards, Villages/Sectors, Verified date).
  - Full parsed archival Markdown rendered cleanly (tables, headings, lists, blockquotes).
  - **"Related records (Vector similarity)"** list at the bottom fetched via LanceDB nearest-neighbor search so users can jump laterally across the archive.

### 4. Wards & Officers Lookup View
- Instant filter box ("Type a village, sector, ward number, or officer name — e.g. Bhangrola, Sector 83, Ward 9, Kasan").
- Displays Ward 1–20 grid/table: clicking any ward reveals its exact villages & sectors, Zone & Division, Elected Councillor, Executive Engineer, Assistant Engineer, Junior Engineer (with phone & email), plus a one-click "Search archival records for Ward N" link.

### 5. Archive Index View
- Browseable catalog of all parsed text records by collection, with category counts, search-within-index filter, and a transparent **Engine Telemetry** strip showing Vector DB (`LanceDB`), Embedding Model (`Xenova/bge-small-en-v1.5`, 384-dim), and Full-Text Engine (`SQLite FTS5 BM25`).

## Quality & Accessibility Floor
- Keyboard accessible (`⌘K` or `/` focuses search, `ArrowDown`/`ArrowUp`/`Enter` navigates suggestions, `Escape` closes drawer/dropdown).
- Visible focus rings (`outline: 2px solid #1B4D7E; outline-offset: 2px`).
- Responsive down to 375px mobile width.
- Respects `prefers-reduced-motion`.
