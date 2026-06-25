# Body Type Filter

**Date:** 2026-06-25
**Status:** Draft

## Problem

Users cannot filter listings by body type (sedan, hatchback, SUV, etc.). Yad2's feed API does not return a structured body type field, but the `subModel` text often embeds this information (e.g., `"Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)"`).

## Approach

Parse body type from the `subModel` text at scrape time, store it on the listing, and expose it as a client-side facet filter (same pattern as fuel type and gearbox chips).

Body type is **not** a search-level server filter (not sent to Yad2, not stored on the `searches` table). It is a post-fetch attribute: parsed from scraped data, stored on `listing_history`, returned in the API response, and filtered client-side via toggle chips.

## Body Type Enum

Normalized lowercase English values:

| Value | Hebrew keywords | English keywords |
|-------|----------------|-----------------|
| `sedan` | סדאן | sedan |
| `hatchback` | האצ'בק, האצ׳בק | hatchback, HB |
| `suv` | קרוסאובר, ג'יפ, ג׳יפ | SUV, crossover |
| `wagon` | סטיישן, טורר | station, SW, wagon, touring |
| `coupe` | קופה, קופא | coupe |
| `convertible` | קבריולה, קבריולט | convertible, cabrio |
| `minivan` | מיניוון | minivan, MPV |
| `pickup` | טנדר | pickup |

Unknown/unparseable submodels produce `""` (empty string). No guessing.

Matching rules:
- Case-insensitive
- Handle both geresh (`'`) and modified letter (`׳`) for Hebrew
- First match wins (order: most specific first)
- Short tokens (`SW`, `HB`, `MPV`, `SUV`) require word boundaries (whitespace or string start/end) to avoid false positives (e.g., `"SKYVIEW"` must not match `SW`)
- `cross` is not a standalone keyword — only `crossover` and `קרוסאובר` match, to avoid false positives on model names like `"CrossBlue"`

## Changes by Layer

### 1. New package: `internal/bodytype/`

**`bodytype.go`** — single exported function:

```go
func Parse(subModel string) string
```

Returns one of the enum values or `""`. Pure function, no side effects. The keyword list and matching logic live here.

**`bodytype_test.go`** — table-driven tests covering:
- Each body type with Hebrew and English variants
- Mixed-case, geresh variants
- Real Yad2 submodel strings (from production data)
- Empty/garbage input returns `""`
- Word boundary edge cases (e.g., `"SKYVIEW"` does not match `SW`)

### 2. Model: `internal/model/listing.go`

Add `BodyType string` field to `RawListing` struct (after `SubModelID`).

### 3. Parser: `internal/fetcher/yad2/parser.go`

After extracting `SubModel` from the feed item, call `bodytype.Parse(subModel.Text)` and set `BodyType` on the `RawListing`.

### 4. Database migration: `migrations/000028_add_body_type.{up,down}.sql`

```sql
-- up
ALTER TABLE listing_history ADD COLUMN body_type TEXT NOT NULL DEFAULT '';

-- down
ALTER TABLE listing_history DROP COLUMN body_type;
```

Default `''` means existing rows are treated as unknown — no backfill needed. New listings get their body type populated on insert.

### 5. Storage layer

**`storage/interfaces.go`** — add `BodyType string` to `ListingRecord`.

**`storage/postgres/listings.go`**:
- Add `body_type` to the `INSERT` column list in `upsertListingSQL`
- Add `body_type` to `scanListingRow`
- Add `body_type` to query SELECT lists where `ListingRecord` is scanned

### 6. API layer: `internal/api/listings.go`

Add `BodyType string` field to `listingResponse` JSON struct:

```go
BodyType string `json:"body_type,omitempty"`
```

Map it in the response builder from `ListingRecord.BodyType`.

### 7. Frontend

**`web/src/lib/api.ts`** — add `body_type?: string` to `Listing` interface.

**`web/src/hooks/useListingFilters.ts`**:
- Add `bodyTypes: string[]` to `ListingFilters` (empty = any)
- Add `bodyTypes: string[]` to `ListingFacets`
- In `deriveFacets()`: collect distinct `body_type` values (skip empty)
- In `filterListings()`: if `f.bodyTypes.length > 0`, exclude listings whose `body_type` is not in the set
- Update `activeFilterCount()` to count `bodyTypes`
- Update `EMPTY_FILTERS` with `bodyTypes: []`

**`web/src/components/ListingsFilterBar.tsx`**:
- Add body type toggle chips (same pattern as fuel/gearbox chips)
- Display Hebrew labels for the chips using a display map:
  - sedan → סדאן, hatchback → האצ'בק, suv → SUV, wagon → סטיישן, coupe → קופה, convertible → קבריולה, minivan → מיניוון, pickup → טנדר

### 8. Tests

| File | What to test |
|------|-------------|
| `internal/bodytype/bodytype_test.go` | Parse function: all enum values, Hebrew/English, edge cases, empty input |
| `internal/fetcher/yad2/parser_test.go` | BodyType field populated after parsing feed items |
| `internal/filter/filter_test.go` | (no change — body type is client-side only) |
| `web/src/hooks/useListingFilters.test.ts` | facet derivation includes body types; filtering by body type works |
| `web/src/components/ListingsFilterBar.test.tsx` | body type chips render and toggle correctly |

## What This Does NOT Change

- **Search creation/update API** — body type is not a search-level filter
- **Yad2 URL builder** — Yad2 has no body type query param
- **Post-fetch filter (`internal/filter/`)** — body type filtering is client-side
- **Enricher** — body type comes from the feed, not the detail page
- **Notifications/Telegram** — body type is informational, not a filter criterion for alerts
- **Scoring** — body type does not affect fitness/deal scores
