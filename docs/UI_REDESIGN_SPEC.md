# Nagar Archive — Unified UI/UX Redesign Specification (Phase B Contract)

Synthesized from the 6-agent UI Design Studio (`_workspace/ui-studio/01..06`). All Phase B implementation agents (`Agents 220–224`) MUST use these EXACT CSS custom properties, class names, `data-*` hooks, and URL contracts so `styles.css`, all 6 Go templates, and `app.js` integrate cleanly on the first compile.

---

## 1. Typography (Google Fonts + Tokens)

Load in `internal/web/templates/layout.html`:
```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,400..700;1,14..32,400..700&amp;family=JetBrains+Mono:wght@500&amp;family=Plus+Jakarta+Sans:wght@600..700&amp;display=swap">
```

- `--indic`: `"Noto Sans Devanagari", "Nirmala UI", "Kohinoor Devanagari"`
- `--display`: `"Plus Jakarta Sans", system-ui, var(--indic), sans-serif` (Wordmark, H1–H3, result titles)
- `--body`: `"Inter", system-ui, var(--indic), sans-serif` (Body, UI, snippets, prose, tables, dates, pills)
- `--mono`: `"JetBrains Mono", ui-monospace, Consolas, monospace` (Document IDs, PIDs, `⌘K` keycap only)
- Type scale:
  - Hero H1 (`.hero-title`): `clamp(1.875rem, 3.6vw, 2.75rem)`, weight `700`, line-height `1.12`, tracking `-0.03em`
  - Page / Record H1 (`h1`, `#drawer-title`): `clamp(1.5rem, 2.4vw, 2rem)`, weight `700`, line-height `1.2`, tracking `-0.025em`
  - Section H2: `clamp(1.125rem, 1.05rem + 0.2vw, 1.25rem)`, weight `600`, line-height `1.3`, tracking `-0.015em`
  - Result / Card Title H3: `clamp(1.0625rem, 1.025rem + 0.1vw, 1.125rem)`, weight `600`, line-height `1.35`, tracking `-0.01em`
  - Body / Snippets / Brief answer: `0.9375rem` (15px), line-height `1.6`, max-width `68ch`
  - Record prose (`.prose`): `clamp(0.96875rem, 0.95rem + 0.05vw, 1rem)`, line-height `1.7`, max-width `68ch`
  - Tables: `0.875rem` (14px), `font-variant-numeric: tabular-nums`
  - Badges / metadata / dates: `0.8125rem` (13px), weight `500`, `font-variant-numeric: tabular-nums`, sentence case (NEVER ALL-CAPS tracked eyebrows)

---

## 2. Dual-Theme Color & Surface System (WCAG AA Verified >= 5.1:1 text, >= 3.65:1 UI)

### Light (`:root`)
- `--bg: #FAFAFB` | `--surface: #FCFCFD` | `--surface-raised: #FFFFFF` | `--surface-muted: #F1F3F6`
- `--border: #D8DDE1` | `--border-strong: #737C88`
- `--ink: #141923` | `--ink-secondary: #4B5565` | `--ink-tertiary: #5B6471`
- `--accent: #0F6868` | `--accent-hover: #085454` | `--accent-subtle: #E1F2EF` | `--accent-ink: #FFFFFF`
- `--highlight-bg: #FFF0C4` | `--highlight-ink: #764900` (snippet `<mark>` & active deadline/date highlights)
- `--verified: #13763A` | `--verified-bg: #E8F6EB`
- `--warn: #873E17` | `--warn-bg: #FCECDD`
- `--shadow-sm: 0 1px 3px rgb(20 25 35 / 9%)`
- `--shadow-md: 0 8px 24px rgb(20 25 35 / 12%)`
- `--shadow-drawer: -8px 0 36px rgb(20 25 35 / 16%)`
- Radii: `--radius-sm: 5px`, `--radius-md: 9px`, `--radius-lg: 14px`, `--radius-pill: 999px`

### Dark (`:root[data-theme="dark"]` AND `@media (prefers-color-scheme: dark) { :root:not([data-theme]) { ... } }`)
- `--bg: #0C1118` | `--surface: #131A24` | `--surface-raised: #1B2432` | `--surface-muted: #101720`
- `--border: #263244` | `--border-strong: #81909F`
- `--ink: #EDF2F7` | `--ink-secondary: #B6C2CF` | `--ink-tertiary: #94A3B8`
- `--accent: #74D5CB` | `--accent-hover: #A2E7DF` | `--accent-subtle: #163536` | `--accent-ink: #0B2424`
- `--highlight-bg: #382E1A` | `--highlight-ink: #F6CF77`
- `--verified: #78DA97` | `--verified-bg: #193225`
- `--warn: #F4BA84` | `--warn-bg: #3D2B21`
- `--shadow-sm: 0 1px 3px rgb(3 8 15 / 35%)`
- `--shadow-md: 0 8px 24px rgb(3 8 15 / 48%)`
- `--shadow-drawer: -8px 0 36px rgb(3 8 15 / 60%)`

### Zero-FOUC Theme Script in `<head>` (before CSS)
Reads `localStorage.getItem("nagar.theme")` inside `try/catch`; if `"light"` or `"dark"`, sets `document.documentElement.dataset.theme = v`. Theme toggle button (`[data-theme-toggle]`) cycles cleanly or switches between Light / Dark / System (`data-theme-state`), persisting in `localStorage["nagar.theme"]` (removing key on System).

---

## 3. Canonical URL & Date Sorting Contract

- `/search` & `/api/search` & `/api/ask`:
  - `q`, `municipality`, `mode` (`hybrid` | `vector` | `keyword`), `sort` (`relevance` | `date_desc` | `date_asc`), `category`, `doc_type`, `ward`.
- `/index` & `/api/documents`:
  - `municipality`, `category`, `doc_type`, `ward`, `sort` (`date_desc` | `date_asc` | `title`).
- `/index` also accepts `q` for text filtering, with or without JavaScript.
- Show full publication dates first in either chronological sort, then partial dates, then undated records. Search sort changes the order of matches, not the order in which the brief cites its evidence.

---

## 4. Shared HTML Structure, Class Names & JS `data-*` Contracts

### Header (`layout.html`)
- `<header class="mast">` (`60px` sticky, `--surface`, `1px solid var(--border)` bottom)
- `.mast-inner`:
  - `.brand` (24px ward-map monogram SVG + `Nagar Archive`)
  - `<details class="municipality" data-city-switcher>`:
    - `<summary class="city-trigger">` shows `<span class="city-name">{{.Municipality.ShortName}}</span>` + chevron SVG (ALWAYS visible even on 375px mobile, ellipsized cleanly if narrow!)
    - `.city-menu`:
      - `<div class="city-search-wrap" data-js hidden><input type="search" class="city-filter" data-city-filter placeholder="Filter 21 municipalities..." aria-label="Filter municipalities"></div>`
      - Pinned `<a class="city-option city-pinned" data-city-link data-city-slug="all" href="/?municipality=all">` (`All 21 municipalities`)
      - `<div class="city-list" data-city-list>`: renders all `.Municipalities` with `data-city-item` & `data-city-terms="{{.Name}} {{.ShortName}} {{.Slug}} {{.State}}"` + `data-city-link` (so `app.js` preserves current route `/search`/`/wards`/`/index` & query params when switching cities!) + ward count pill (`{{.WardsCount}} wards`) + `Rajasthan` badge for Bhiwadi.
      - `<p class="city-empty" data-city-empty hidden>No municipalities match your filter.</p>`
  - `<nav class="mast-nav" aria-label="Main navigation">`: `Search`, `Wards & officers`, `Archive index` (preserving `?municipality={{.Municipality.Slug}}`, active link gets `aria-current="page"` and 2px bottom `--accent` border).
  - `<button type="button" class="theme-toggle" data-theme-toggle data-js hidden aria-label="Switch color theme" title="Switch color theme">` (sun/moon/monitor SVG icon + subtle label on desktop or crisp icon button).

### Unified Command Search Bar (`{{define "search-form"}}` in `layout.html`)
- `<form class="command" action="/search" method="get" role="search" data-search-form>`
- Hidden inputs: `municipality`, `sort` (if set), `category`, `doc_type`, `ward`
- `.search-line`:
  - Search magnifier SVG
  - `<input id="archive-query" name="q" type="search" data-focus ...>`
  - `<button type="button" class="clear-btn" data-clear-query aria-label="Clear search" hidden>×</button>`
  - `<kbd class="shortcut" data-shortcut>⌘K</kbd>`
  - `<button type="submit" class="btn-primary">Search</button>`
- `#suggestions.suggestions` (raised overlay with `--shadow-md`, typed badge per suggestion row: `Ward`, `Village`, `Sector`, `Officer`, `Record`, `Prompt`)
- `.command-controls` (below input on Home; compact inline on docked Search):
  - Segmented radio group `.segmented` for `mode`: `Hybrid` (`hybrid`), `Semantic` (`vector`), `Exact` (`keyword`)
  - On Home (or docked form), segmented radio group `.segmented` for `sort`: `Relevance` (`relevance`), `Newest first` (`date_desc`), `Oldest first` (`date_asc`).

### Home Search Desk (`home.html`)
- Centered `.desk-hero` (`max-width: 780px; margin-inline: auto`):
  - `<h1 class="hero-title">Search the {{.Municipality.ShortName}} civic archive</h1>` (or `Search all 21 municipal archives` when `all`)
  - `<p class="hero-lead">...`
  - `{{template "search-form" .}}`
  - `.prompt-grid` (2-column grid of clickable `.prompt-card` links for `.PromptExamples` with crisp search-arrow icon)
- Horizontal `.overview-strip` (`max-width: 1120px`): compact ruled bar showing Records, Indexed passages (chunks), Wards, Officers, and clickable Helpdesk / Toll-free phone links (only when municipality has them!).
- `.coverage` sections: **Explore collections** (`.collection-grid` 3-col cards) and **21 indexed municipalities** (`.city-grid` compact directory cards).

### Search Workspace (`search.html`)
- `.dock`: compact sticky/top search bar strip (`{{template "search-form" .}}` without redundant duplicate sort controls).
- `.workspace`: `260px minmax(0, 1fr)` on desktop (`>=920px`), stacked on mobile with collapsible `<details class="mobile-filters">` wrapping the filter rail on `<920px` (actually using ONE responsive `.rail-wrap` `<details class="rail-drawer" open>` on desktop, closed on mobile via tiny JS/CSS or clean responsive rail so there's zero duplicate accessibility tree!).
- The facet form submits only through `Apply filters`, so changing a field does not navigate away. It preserves `q`, `municipality` and the selected sort.
- `.stream`:
  - **Prompted Archive Brief** (`.brief`, rendered only when `.Brief.Headline != ""` and `.Brief.Headline != "No matching evidence"`):
    - `.brief-head`: `Summary from archive records` pill + `.badge-confidence` (`High`/`Medium`/`Low confidence`) + search time pill (`{{.TookMs}} ms`) + anchor link `Jump to {{.Total}} records` (`#results`)
    - `<h1>{{.Headline}}</h1>`
    - Answer paragraphs with clickable superscript inline citations `{{renderParagraph . $sources}}`
    - Key takeaways list (`.brief-points`)
    - Structured **Contacts & actions** (`.action-grid`: `.contact-card` for contacts with phone/email SVG icons, `.portal-btn` for official portals with external-link SVG, `.fact-tile` for deadlines/amounts/dates/wards)
    - Compact **Sources** strip + **Follow-up questions** (`.followup-list`)
  - **Results Header & Sort Toolbar** (`#results.results-bar`):
    - Left: `Showing {{len .Hits}} of {{.Total}} matches` (or `{{.Total}} matches`)
    - Right: **Sort segmented links** (`.sort-pills`): `Relevance` (`sort=relevance`), `Newest first` (`sort=date_desc`), `Oldest first` (`sort=date_asc`), preserving `q`, `municipality`, `mode`, `category`, `doc_type`, `ward`, with `aria-current="true"` on the active sort!
  - **Ranked Match Cards** (`.match-list` -> `article.match-card`):
    - Top row `.match-top`: clickable `<h3><a data-open-doc href="/doc/{{.Municipality}}/{{.DocID}}">{{.Title}}</a></h3>` + prominent date badge `<time class="date-pill" datetime="{{.Date}}">{{formatDate .Date}}</time>`
    - Meta row `.match-meta`: municipality pill (when `all`), collection pill, up to 4 ward pills (`+N more` summary if >4), and `Section: {{.Heading}}` when present
    - Snippet `<p class="match-snippet" data-snippet>{{.Snippet}}</p>`
    - Footer `.match-foot`: source portal (`{{.SourcePortal}}`) on left, ONE clean retrieval pill (`.signal-pill`) on right (e.g., `Hybrid · 86% semantic` or `Keyword match`, replacing the old 6-row `<dl class="signals">`!)

### Document Reader Drawer (`layout.html` + `app.js`) & Full Page (`document.html`)
- `<dialog id="reader" class="reader-dialog">`: right-docked `min(840px, 94vw)`, full screen under 640px, single sticky `.drawer-toolbar` with Document ID pill (`.doc-id`), `Copy text` (`[data-copy]`), `Full page`, `Official source`, and `×` Close (`[data-close]`, `aria-label="Close document reader"`).
- Compact 2-column `.meta-card` (Collection, Document type, Date via `formatDate`, Department) right under title, plus collapsible `<details class="provenance-more"><summary>Full provenance & geography</summary>...</details>` and collapsible `<details class="contents"><summary>Contents</summary>...</details>` so `.prose` starts immediately above the fold!
- GFM tables wrapped in `.table-scroll` (rounded border, tinted sticky header, alternating subtle zebra rows).
- Related records as clean `.related-grid` cards with similarity pill.

### Wards & Officers (`wards.html`)
- Live search input `[data-filter]` with clear button & live status pill `[data-filter-status]`.
- Segmented tablist (`[data-tab-list]`): `Ward register ({{len .Wards}})` (`[data-tab="wards"]`) | `Officers directory ({{len .Officers}})` (`[data-tab="officers"]`) — no-JS shows both cleanly!
- Upgraded `.ward-card` `<details>`: summary shows `Ward N` pill, city pill (when `"all"`), ward name/zone preview, councillor name at a glance, and chevron; expanded body uses 2-column grid (Geography on left; Councillor + EE + AE + all `.JuniorEngineers` contact cards on right) + `"Search Ward N records"` button.

### Archive Index (`index.html`)
- Compact wrapping `.telemetry-strip` of small pills (Vector DB, Embedding model 384d, FTS5, Docs, Chunks, Words).
- GET `.index-toolbar` form: live search input `[data-filter]` + Collection `<select name="category" data-collection>` + **Sort by date `<select name="sort" data-index-sort>` (`Newest first` `date_desc`, `Oldest first` `date_asc`, `Title A–Z` `title`)** + no-JS Apply button!
- Clean `.register-table` with `<tr data-row data-category="..." data-date="{{.Date}}" data-title="{{.Title}}" data-terms="...">`: Title (`data-open-doc`), Municipality (when `"all"`), Collection pill, Document type, formatted `<time datetime="{{.Date}}">{{formatDate .Date}}</time>`, and right-aligned Word count!
