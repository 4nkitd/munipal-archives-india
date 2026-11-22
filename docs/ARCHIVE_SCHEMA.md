# Municipal Archives India — Archival Text Schema

This document defines the canonical plain-text archival format for all municipal corporations in this repository.

## Directory Structure

Every municipality lives under `archives/<municipality_slug>/`:

```
archives/
  manesar/
    municipality.json               # Municipality profile & metadata
    wards.json                      # Canonical Ward 1..20 -> villages/sectors/zone/division/councillor mapping
    officers.json                   # Canonical Officers & Ward Engineering responsibility directory
    01-governance/                  # Formation, city profile, leadership, elected council, elections
    02-wards-geography/             # Wardbandi notifications, zones, area maps, regularized colonies, villages/sectors
    03-officers-directory/          # Officers roster, ward engineers, NAKSHA team, helplines & offices
    04-notifications-orders/        # Official notifications, Area Sabha, GRAP, Account Code 2025, Lal Dora PID lists
    05-reference-documents/         # Taxation rates, village Lal Dora archives, Building Code 2017, Engineering, Admin, Connectivity
    06-citizen-services/            # Property tax, RTI Act 2005, Right to Service, grievances, birth/death, marriage, online portals
    07-tenders-works/               # HEWP & eTender notices, RFPs, ponds/roads/tubewells/SWM infrastructure works
    08-finance-legal/               # Budgets, Haryana Municipal Corporation Act 1994, bye-laws, audit & finances
    09-urban-planning-imt/          # IMT Manesar (HSIIDC), GMDA interface, GMUC 2031, GIS, sweeping & environment
```

## Canonical Markdown File Format

Every archival record is a `.md` file with valid YAML frontmatter delimited by `---` lines.
NEVER store raw PDFs in `archives/`. Download PDFs to a temporary directory under `_workspace/tmp/`, extract their text (`pdftotext -layout` or `pdftotext`), structure the real information into clean `.md` files, and delete temporary PDFs when done.

### Required YAML Frontmatter Fields

```yaml
---
id: "mcm-wards-final-wardbandi-2023"
municipality: "manesar"
title: "Final notification of wardbandi (20 wards) — Municipal Corporation Manesar"
category: "wards-geography"
subcategory: "ward-delimitation"
doc_type: "notification"
date: "2023-11-03"
source_url: "https://mcmanesar.in/sites/default/files/2025-09/W1_0.pdf"
source_portal: "mcmanesar.in"
original_filename: "W1_0.pdf"
department: "Town Planning & Election Branch"
wards: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20]
sectors: ["Sector 1 IMT", "Sector 8 IMT", "Sector 83", "Sector 84"]
villages: ["Manesar", "Kasan", "Bhangrola", "Hayatpur"]
tags: ["wardbandi", "ward-boundaries", "delimitation", "election"]
contacts:
  - name: "Municipal Corporation Manesar Helpdesk"
    role: "Helpdesk"
    phone: "0124-2012232"
    email: "cmc-manesar@ulbharyana.gov.in"
action_url: "https://mcmanesar.in/wards"
summary: "One or two factual sentences summarizing what this record contains and when a citizen needs it."
verified_at: "2026-11-22"
---
```

### Allowed Values

- `category` (MUST be one of these exact slugs):
  - `governance`
  - `wards-geography`
  - `officers-directory`
  - `notifications-orders`
  - `reference-documents`
  - `citizen-services`
  - `tenders-works`
  - `finance-legal`
  - `urban-planning-imt`
- `doc_type` (choose the closest fit):
  - `notification` | `order` | `policy` | `directory` | `service-guide` | `tender` | `rfp` | `act-bylaw` | `ward-record` | `property-record` | `rti-disclosure` | `rts-timeline` | `budget` | `map-index` | `public-notice` | `profile` | `report`
- `id`: Globally unique kebab-case identifier. Use the municipality slug as a prefix for new municipalities. Keep existing IDs stable.
- `date`: Official publication date at its actual precision: `YYYY-MM-DD`, `YYYY-MM`, or `YYYY`. Leave it empty if the publication date is unknown. Do not substitute the date checked or assume January 1.
- `verified_at`: Date the official source was checked. A verification date does not make a historical ward list or officer appointment current.
- `wards`: Positive ward numbers from the cited municipal source and its edition. Use `[]` when the record is municipality-wide or does not identify wards. Do not assume statewide numbering or infer ward boundaries from ward office addresses.
- `sectors`: Array of normalized sector names (e.g. `"Sector 83"`, `"IMT Sector 8"`). Empty `[]` if none.
- `villages`: Array of normalized village names (e.g. `"Bhangrola"`, `"Kasan"`, `"Manesar"`). Empty `[]` if none.
- `contacts`: Optional array of `{ name, role, phone?, email? }` objects when the record names responsible officials, PIOs, helplines, or tender authorities.
- `action_url`: Optional official online portal URL where a citizen completes the service or views the source.

Individual `wards.json` rows may include `source_url` and `verified_at`. Link the official ward source, including a page fragment if known. Do not infer a boundary from an office list, an election result, or a GIS label alone.

### Markdown Body Standards

1. Start with an H1 `# <Title>` matching the frontmatter title.
2. Organize content with clear H2 `##` and H3 `###` headings so the section-aware chunker creates clean semantic chunks (aim for self-contained sections of 150–450 words).
3. Preserve concrete facts: exact dates, notification numbers, officer names, phone numbers, emails, rupee amounts, timelines, ward numbers, village names, sector numbers, fees, eligibility rules, and verbatim statutory clauses where useful.
4. Use Markdown tables for structured schedules (officers, wards, property tax slabs, RTS timelines, tenders, Lal Dora village summaries).
5. If a PDF is image-only/scanned and `pdftotext` yields no text, record the official metadata, document title, source URL, and all verified details from the portal page & authoritative cross-references clearly, noting `[Scanned map/PDF — visual source linked at source_url]` instead of inventing numbers.
6. Write crisp, plain, factual prose. No marketing puffery, no filler, no em dashes. Quote real numbers and real citations.
