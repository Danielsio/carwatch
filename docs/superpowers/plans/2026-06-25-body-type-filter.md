# Body Type Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse body type (sedan, hatchback, SUV, etc.) from Yad2 sub-model text, store it on listings, and expose it as a client-side facet filter.

**Architecture:** Body type is extracted at scrape time from the sub-model text via keyword matching, stored as a TEXT column on `listing_history`, returned in the API JSON response, and filtered client-side using toggle chips (same pattern as fuel type / gearbox).

**Tech Stack:** Go (backend), PostgreSQL (migration), React + TypeScript (frontend), Vitest (frontend tests)

## Global Constraints

- Body type is NOT a search-level filter (not stored on `searches`, not sent to Yad2)
- Unknown/unparseable sub-models produce `""` (empty string) — no guessing
- Migration must be backwards-compatible (DEFAULT `''` for existing rows)
- All new code must have tests

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/bodytype/bodytype.go` | `Parse(subModel string) string` — keyword matching |
| Create | `internal/bodytype/bodytype_test.go` | Table-driven tests for Parse |
| Modify | `internal/model/listing.go:5-33` | Add `BodyType string` to `RawListing` |
| Modify | `internal/fetcher/yad2/parser.go:218-254` | Call `bodytype.Parse()` in `itemToListing` |
| Modify | `internal/fetcher/yad2/parser_test.go` | Test BodyType field is set after parsing |
| Create | `migrations/000028_add_body_type.up.sql` | `ALTER TABLE listing_history ADD COLUMN body_type TEXT NOT NULL DEFAULT ''` |
| Create | `migrations/000028_add_body_type.down.sql` | `ALTER TABLE listing_history DROP COLUMN body_type` |
| Modify | `internal/storage/interfaces.go:146-185` | Add `BodyType string` to `ListingRecord` |
| Modify | `internal/storage/postgres/listings.go:39-69` | Add `body_type` to upsert SQL |
| Modify | `internal/storage/postgres/listings.go:84-122` | Add `body_type` to `scanListingRow` |
| Modify | `internal/storage/postgres/listings.go:124-131` | Add `body_type` to `upsertListingArgs` |
| Modify | `internal/storage/postgres/listings.go:268-277` | Add `body_type` to GetListing SELECT |
| Modify | `internal/storage/postgres/listings.go:289-297` | Add `body_type` to ListUserListings SELECT |
| Modify | `internal/storage/postgres/listings.go:417-425` | Add `body_type` to ListSearchListings SELECT |
| Modify | `internal/storage/postgres/listings.go:740-744` | Add `body_type` to ListSaved SELECT |
| Modify | `internal/storage/postgres/notifications_center.go:26-30` | Add `body_type` to NewListingsSince SELECT |
| Modify | `internal/storage/postgres/admin.go:162-166` | Add `body_type` to admin SELECT + scan |
| Modify | `internal/scheduler/pipeline.go:325-362` | Add `BodyType` to `buildRecord` |
| Modify | `internal/api/listings.go:15-49` | Add `BodyType` to `listingResponse` |
| Modify | `internal/api/listings.go:258-284` | Map `BodyType` in single-listing handler |
| Modify | `internal/api/bookmarks.go:227-276` | Map `BodyType` in `toListingResponses` |
| Modify | `internal/api/instant_search.go:240-257` | Map `BodyType` in instant search |
| Modify | `web/src/lib/api.ts:119-153` | Add `body_type?: string` to `Listing` |
| Modify | `web/src/hooks/useListingFilters.ts` | Add `bodyTypes` to filters, facets, filtering |
| Modify | `web/src/hooks/useListingFilters.test.ts` | Test body type filtering and facets |
| Modify | `web/src/components/ListingsFilterBar.tsx` | Add body type toggle chips |
| Modify | `web/src/components/ListingsFilterBar.test.tsx` | Test body type chips render and toggle |

---

### Task 1: Body Type Parser

**Files:**
- Create: `internal/bodytype/bodytype.go`
- Create: `internal/bodytype/bodytype_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `func Parse(subModel string) string` — returns one of `"sedan"`, `"hatchback"`, `"suv"`, `"coupe"`, `"wagon"`, `"convertible"`, `"minivan"`, `"pickup"`, or `""`.

- [ ] **Step 1: Write the failing test**

Create `internal/bodytype/bodytype_test.go`:

```go
package bodytype

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		subModel string
		want     string
	}{
		// Hebrew body types
		{"hebrew sedan", "Comfort סדאן 1.6 (132 כ״ס)", "sedan"},
		{"hebrew hatchback geresh", "Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)", "hatchback"},
		{"hebrew hatchback modified", "Excite האצ׳בק 1.6", "hatchback"},
		{"hebrew suv jeep geresh", "Limited ג'יפ 2.0 (150 כ״ס)", "suv"},
		{"hebrew suv jeep modified", "Sport ג׳יפ 2.5", "suv"},
		{"hebrew crossover", "Active קרוסאובר 1.5 (115 כ״ס)", "suv"},
		{"hebrew wagon station", "Touring סטיישן 2.0 (150 כ״ס)", "wagon"},
		{"hebrew wagon tourer", "Luxury טורר אוט׳ 1.8", "wagon"},
		{"hebrew coupe", "Sport קופה 2.0 (200 כ״ס)", "coupe"},
		{"hebrew coupe alt", "RS קופא 3.0", "coupe"},
		{"hebrew convertible cabrio", "Sport קבריולה 2.0", "convertible"},
		{"hebrew convertible cabriolet", "קבריולט 1.8", "convertible"},
		{"hebrew minivan", "Comfort מיניוון 2.0 (150 כ״ס)", "minivan"},
		{"hebrew pickup", "Limited טנדר 3.0 (190 כ״ס)", "pickup"},

		// English body types
		{"english sedan", "Comfort sedan 1.6", "sedan"},
		{"english hatchback", "Sport hatchback 1.4 TSI", "hatchback"},
		{"english HB word boundary", "Sport HB 1.4", "hatchback"},
		{"english SUV word boundary", "Premium SUV 2.0", "suv"},
		{"english crossover", "Active crossover 1.5", "suv"},
		{"english wagon", "Touring wagon 2.0", "wagon"},
		{"english SW word boundary", "Comfort SW 1.6", "wagon"},
		{"english station", "station 2.0 diesel", "wagon"},
		{"english touring", "Grand Touring 2.5", "wagon"},
		{"english coupe", "Sport coupe 2.0", "coupe"},
		{"english convertible", "Sport convertible 2.0", "convertible"},
		{"english cabrio", "Sport cabrio 1.8", "convertible"},
		{"english minivan", "Comfort minivan 2.4", "minivan"},
		{"english MPV word boundary", "Comfort MPV 2.0", "minivan"},
		{"english pickup", "Limited pickup 3.0", "pickup"},

		// Case insensitivity
		{"mixed case SEDAN", "Comfort SEDAN 1.6", "sedan"},
		{"mixed case Hatchback", "Sport Hatchback 1.4", "hatchback"},
		{"lowercase suv", "Premium suv 2.0", "suv"},

		// Word boundary: short tokens must not match inside words
		{"SW inside word SKYVIEW", "Premium SKYVIEW 2.0", ""},
		{"HB inside word THBIRD", "Premium THBIRD 1.8", ""},
		{"SUV inside word INSUVERABLE", "INSUVERABLE 2.0", ""},

		// No match
		{"empty string", "", ""},
		{"no body type keywords", "Premium אוט׳ 2.0 (165 כ״ס) [2017-2019]", ""},
		{"just trim info", "LUXURY", ""},
		{"engine only", "1.8 (98 כ״ס)", ""},

		// Real Yad2 submodel strings
		{"real yad2 hatchback", "Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)", "hatchback"},
		{"real yad2 no body type", "Premium אוט׳ 2.0 (165 כ״ס) [2017-2019]", ""},
		{"real yad2 sport no type", "Sport אוט׳ 1.8", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.subModel)
			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.subModel, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bodytype/ -v -count=1`
Expected: compilation error — package/function does not exist.

- [ ] **Step 3: Write the implementation**

Create `internal/bodytype/bodytype.go`:

```go
package bodytype

import (
	"regexp"
	"strings"
)

const (
	Sedan       = "sedan"
	Hatchback   = "hatchback"
	SUV         = "suv"
	Coupe       = "coupe"
	Wagon       = "wagon"
	Convertible = "convertible"
	Minivan     = "minivan"
	Pickup      = "pickup"
)

var All = []string{Sedan, Hatchback, SUV, Coupe, Wagon, Convertible, Minivan, Pickup}

type matcher struct {
	bodyType string
	match    func(lower string) bool
}

func contains(substr string) func(string) bool {
	return func(s string) bool { return strings.Contains(s, substr) }
}

func wordBoundary(word string) func(string) bool {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	return func(s string) bool { return re.MatchString(s) }
}

var matchers []matcher

func init() {
	matchers = []matcher{
		// Hatchback (before sedan — more specific)
		{Hatchback, contains("האצ'בק")},
		{Hatchback, contains("האצ׳בק")}, // modified letter geresh ׳
		{Hatchback, contains("hatchback")},
		{Hatchback, wordBoundary("HB")},

		// Sedan
		{Sedan, contains("סדאן")},
		{Sedan, contains("sedan")},

		// SUV / Crossover
		{SUV, contains("קרוסאובר")},
		{SUV, contains("crossover")},
		{SUV, contains("ג'יפ")},
		{SUV, contains("ג׳יפ")}, // modified letter geresh ׳
		{SUV, wordBoundary("SUV")},

		// Wagon / Station / Touring
		{Wagon, contains("סטיישן")},
		{Wagon, contains("טורר")},
		{Wagon, contains("station")},
		{Wagon, contains("touring")},
		{Wagon, contains("wagon")},
		{Wagon, wordBoundary("SW")},

		// Coupe
		{Coupe, contains("קופה")},
		{Coupe, contains("קופא")},
		{Coupe, contains("coupe")},

		// Convertible
		{Convertible, contains("קבריולה")},
		{Convertible, contains("קבריולט")},
		{Convertible, contains("convertible")},
		{Convertible, contains("cabrio")},

		// Minivan
		{Minivan, contains("מיניוון")},
		{Minivan, contains("minivan")},
		{Minivan, wordBoundary("MPV")},

		// Pickup
		{Pickup, contains("טנדר")},
		{Pickup, contains("pickup")},
	}
}

func Parse(subModel string) string {
	if subModel == "" {
		return ""
	}
	lower := strings.ToLower(subModel)
	for _, m := range matchers {
		if m.match(lower) {
			return m.bodyType
		}
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bodytype/ -v -count=1`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bodytype/bodytype.go internal/bodytype/bodytype_test.go
git commit -m "feat: add body type parser for sub-model text

Signed-off-by: Daniel Sionov <kingddd301@gmail.com>"
```

---

### Task 2: Model + Parser Integration

**Files:**
- Modify: `internal/model/listing.go:5-33`
- Modify: `internal/fetcher/yad2/parser.go:218-254`
- Modify: `internal/fetcher/yad2/parser_test.go`

**Interfaces:**
- Consumes: `bodytype.Parse(subModel string) string` from Task 1
- Produces: `RawListing.BodyType` field populated after parsing

- [ ] **Step 1: Add BodyType to RawListing**

In `internal/model/listing.go`, add `BodyType` after `SubModelID`:

```go
SubModelID         int
BodyType           string
Year               int
```

- [ ] **Step 2: Write the failing parser test**

Add to `internal/fetcher/yad2/parser_test.go`:

```go
func TestParseNextData_ExtractsBodyTypeFromSubModel(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-test",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)"},
				"vehicleDates":{"yearOfProduction":2021},
				"engineVolume":1798,
				"price":95000,
				"hand":{"id":2},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "hatchback" {
		t.Errorf("BodyType = %q, want 'hatchback'", listings[0].BodyType)
	}
}

func TestParseNextData_BodyTypeEmptyWhenNoMatch(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-empty",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"Premium אוט׳ 2.0 (165 כ״ס)"},
				"vehicleDates":{"yearOfProduction":2020},
				"price":90000,
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "" {
		t.Errorf("BodyType = %q, want empty string", listings[0].BodyType)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/fetcher/yad2/ -run TestParseNextData_ExtractsBodyType -v -count=1`
Expected: FAIL — BodyType field is always empty because parser doesn't set it.

- [ ] **Step 4: Wire bodytype.Parse into the parser**

In `internal/fetcher/yad2/parser.go`, add import:

```go
"github.com/dsionov/carwatch/internal/bodytype"
```

In the `itemToListing` function, after the HP/gearbox extraction block (after line ~254), add:

```go
listing.BodyType = bodytype.Parse(subModelText)
```

This goes right after the `listing.GearBox = "אוטומט"` block, since `subModelText` is already declared there.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/fetcher/yad2/ -v -count=1`
Expected: all tests PASS, including the two new body type tests.

- [ ] **Step 6: Commit**

```bash
git add internal/model/listing.go internal/fetcher/yad2/parser.go internal/fetcher/yad2/parser_test.go
git commit -m "feat: extract body type from sub-model text during parsing

Signed-off-by: Daniel Sionov <kingddd301@gmail.com>"
```

---

### Task 3: Database Migration + Storage Layer

**Files:**
- Create: `migrations/000028_add_body_type.up.sql`
- Create: `migrations/000028_add_body_type.down.sql`
- Modify: `internal/storage/interfaces.go:146-185`
- Modify: `internal/storage/postgres/listings.go` (multiple locations)
- Modify: `internal/storage/postgres/notifications_center.go:26-30`
- Modify: `internal/storage/postgres/admin.go:162-196`
- Modify: `internal/scheduler/pipeline.go:325-362`

**Interfaces:**
- Consumes: `RawListing.BodyType` from Task 2
- Produces: `ListingRecord.BodyType` persisted and read from DB

- [ ] **Step 1: Create migration files**

Create `migrations/000028_add_body_type.up.sql`:
```sql
ALTER TABLE listing_history ADD COLUMN body_type TEXT NOT NULL DEFAULT '';
```

Create `migrations/000028_add_body_type.down.sql`:
```sql
ALTER TABLE listing_history DROP COLUMN body_type;
```

- [ ] **Step 2: Add BodyType to ListingRecord**

In `internal/storage/interfaces.go`, add `BodyType string` after `SubModelID int` in the `ListingRecord` struct:

```go
SubModelID   int
BodyType     string
Year         int
```

- [ ] **Step 3: Update upsert SQL**

In `internal/storage/postgres/listings.go`, update `upsertListingSQL`:

In the INSERT column list, add `body_type` after `sub_model_id`:
```
(token, chat_id, search_id, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price, km, hand, city, page_link, image_url, engine_volume, horse_power, engine_type, gear_box, description, is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, posted_at)
```

Update VALUES to add `$9` for body_type and shift all subsequent params by 1 (up to `$29`):
```
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
```

In the ON CONFLICT DO UPDATE block, add after the `sub_model_id` update:
```sql
body_type = CASE WHEN EXCLUDED.body_type != '' THEN EXCLUDED.body_type ELSE listing_history.body_type END,
```

- [ ] **Step 4: Update scanListingRow**

In the `scanListingRow` function, add `&l.BodyType` after `&l.SubModelID` in the Scan call:

```go
if err := sc.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model, &l.SubModel, &l.SubModelID, &l.BodyType,
    &l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL,
    &l.EngineVolume, &l.HorsePower, &l.EngineType, &l.GearBox, &l.Description,
    &ic, &fs, &mp, &cs, &ds, &bp, &l.FirstSeenAt, &postedAt, &removedAt); err != nil {
```

- [ ] **Step 5: Update upsertListingArgs**

Add `r.BodyType` after `r.SubModelID`:

```go
func upsertListingArgs(r storage.ListingRecord) []any {
	return []any{
		r.Token, r.ChatID, r.SearchID, r.SearchName, r.Manufacturer, r.Model, r.SubModel, r.SubModelID, r.BodyType, r.Year, r.Price,
		r.Km, r.Hand, r.City, r.PageLink, r.ImageURL,
		r.EngineVolume, r.HorsePower, r.EngineType, r.GearBox, r.Description,
		storage.ListingCommercialToSQL(r.IsCommercial),
		r.FitnessScore, r.MedianPrice, r.CohortSize, r.DealScore, r.BasePrice, r.FirstSeenAt.UTC(), r.PostedAt,
	}
}
```

- [ ] **Step 6: Update all SELECT queries that feed scanListingRow**

Add `body_type` after `sub_model_id` in every SELECT that uses `scanListingRow`. There are 5 locations:

1. `GetListing` (~line 270):
```sql
SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price,
```

2. `ListUserListings` (~line 290):
```sql
SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price,
```

3. `ListSearchListings` (~line 418):
```sql
SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price,
```

4. `ListSaved` (bookmarks, ~line 741):
```sql
SELECT lh.token, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.body_type, lh.year, lh.price,
```

5. `NewListingsSince` (notifications_center.go, ~line 27):
```sql
SELECT lh.token, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.body_type, lh.year, lh.price,
```

- [ ] **Step 7: Update admin query**

In `internal/storage/postgres/admin.go` (~line 163), add `body_type` to the SELECT:
```sql
SELECT lh.token, lh.chat_id, lh.search_id, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.body_type, lh.year, lh.price,
```

And in the Scan call (~line 192), add `&r.BodyType` after `&r.SubModelID`:
```go
&r.Manufacturer, &r.Model, &r.SubModel, &r.SubModelID, &r.BodyType, &r.Year, &r.Price,
```

- [ ] **Step 8: Update pipeline buildRecord**

In `internal/scheduler/pipeline.go` `buildRecord` function, add `BodyType` after `SubModelID`:

```go
rec := storage.ListingRecord{
    Token:        listing.Token,
    ChatID:       params.ChatID,
    SearchID:     params.SearchID,
    SearchName:   params.SearchName,
    Manufacturer: listing.Manufacturer,
    Model:        listing.Model,
    SubModel:     listing.SubModel,
    SubModelID:   listing.SubModelID,
    BodyType:     listing.BodyType,
    Year:         listing.Year,
```

- [ ] **Step 9: Run all Go tests**

Run: `go test ./... -count=1 2>&1 | tail -30`
Expected: all tests PASS (database tests may skip if no test DB — that's OK).

- [ ] **Step 10: Commit**

```bash
git add migrations/000028_add_body_type.up.sql migrations/000028_add_body_type.down.sql \
  internal/storage/interfaces.go internal/storage/postgres/listings.go \
  internal/storage/postgres/notifications_center.go internal/storage/postgres/admin.go \
  internal/scheduler/pipeline.go
git commit -m "feat: add body_type column and wire through storage layer

Signed-off-by: Daniel Sionov <kingddd301@gmail.com>"
```

---

### Task 4: API Response + Frontend

**Files:**
- Modify: `internal/api/listings.go:15-49`
- Modify: `internal/api/listings.go:258-284`
- Modify: `internal/api/bookmarks.go:227-276`
- Modify: `internal/api/instant_search.go:240-257`
- Modify: `web/src/lib/api.ts:119-153`
- Modify: `web/src/hooks/useListingFilters.ts`
- Modify: `web/src/hooks/useListingFilters.test.ts`
- Modify: `web/src/components/ListingsFilterBar.tsx`
- Modify: `web/src/components/ListingsFilterBar.test.tsx`

**Interfaces:**
- Consumes: `ListingRecord.BodyType` from Task 3
- Produces: `body_type` field in API JSON; client-side filter chips

- [ ] **Step 1: Add BodyType to API response struct**

In `internal/api/listings.go`, add to `listingResponse` struct after `SubModel`:

```go
SubModel     string   `json:"sub_model,omitempty"`
BodyType     string   `json:"body_type,omitempty"`
Year         int      `json:"year"`
```

- [ ] **Step 2: Map BodyType in all response builders**

In `internal/api/listings.go` single-listing handler (~line 258), add after `SubModel`:
```go
BodyType:     l.BodyType,
```

In `internal/api/bookmarks.go` `toListingResponses` (~line 244), add after `SubModel`:
```go
BodyType:     l.BodyType,
```

In `internal/api/instant_search.go` (~line 244), add after `SubModel`:
```go
BodyType:     l.BodyType,
```
Note: instant_search builds from `model.RawListing`, which now has `BodyType`.

- [ ] **Step 3: Run Go tests**

Run: `go test ./internal/api/... -count=1 -v 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 4: Add body_type to frontend Listing type**

In `web/src/lib/api.ts`, add after `sub_model`:

```typescript
sub_model?: string;
body_type?: string;
year: number;
```

- [ ] **Step 5: Write failing frontend filter tests**

Add to `web/src/hooks/useListingFilters.test.ts`:

```typescript
it("filters by selected body types", () => {
  const items = [
    listing({ body_type: "sedan" }),
    listing({ body_type: "hatchback" }),
    listing({ body_type: undefined }),
  ];
  expect(filterListings(items, f({ bodyTypes: ["sedan"] }))).toHaveLength(1);
  expect(
    filterListings(items, f({ bodyTypes: ["sedan", "hatchback"] })),
  ).toHaveLength(2);
});
```

Add to the `deriveFacets` describe block:

```typescript
it("collects distinct body types (skipping empty)", () => {
  const items = [
    listing({ body_type: "sedan" }),
    listing({ body_type: "hatchback" }),
    listing({ body_type: "sedan" }),
    listing({ body_type: undefined }),
  ];
  const facets = deriveFacets(items);
  expect(facets.bodyTypes).toEqual(["hatchback", "sedan"]);
});
```

Add to the `activeFilterCount` describe block — update the existing count test to also verify bodyTypes is counted:

```typescript
it("counts bodyTypes as active", () => {
  expect(
    activeFilterCount(f({ bodyTypes: ["sedan"] })),
  ).toBe(1);
});
```

- [ ] **Step 6: Run frontend tests to verify they fail**

Run: `cd web && npx vitest run src/hooks/useListingFilters.test.ts 2>&1 | tail -20`
Expected: FAIL — `bodyTypes` property doesn't exist on filter types.

- [ ] **Step 7: Update useListingFilters.ts**

In `web/src/hooks/useListingFilters.ts`:

Add `bodyTypes` to `ListingFilters`:
```typescript
/** Selected body types (body_type); empty = any. */
bodyTypes: string[];
```

Add `bodyTypes` to `EMPTY_FILTERS`:
```typescript
bodyTypes: [],
```

Add `bodyTypes` to `activeFilterCount`:
```typescript
if (f.bodyTypes.length) n++;
```

Add `bodyTypes` to `ListingFacets`:
```typescript
bodyTypes: string[];
```

Add to `deriveFacets`:
```typescript
const bodyTypes = new Set<string>();
```
Inside the loop:
```typescript
if (l.body_type) bodyTypes.add(l.body_type);
```
In the return:
```typescript
bodyTypes: [...bodyTypes].sort(),
```

Add to `filterListings` predicate, after the gearboxes check:
```typescript
if (
  f.bodyTypes.length > 0 &&
  (!l.body_type || !f.bodyTypes.includes(l.body_type))
)
  return false;
```

- [ ] **Step 8: Run frontend filter tests**

Run: `cd web && npx vitest run src/hooks/useListingFilters.test.ts 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 9: Write failing ListingsFilterBar test**

Add to `web/src/components/ListingsFilterBar.test.tsx`:

Update the `facets` object to include `bodyTypes`:
```typescript
const facets: ListingFacets = {
  fuels: ["בנזין", "היברידי"],
  gearboxes: ["אוטומטית"],
  bodyTypes: ["sedan", "hatchback"],
  priceMax: 200000,
  yearMin: 2015,
  yearMax: 2023,
  kmMax: 150000,
};
```

Add test:
```typescript
it("renders body type chips with Hebrew labels", () => {
  setup();
  expect(screen.getByRole("button", { name: "סדאן" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "האצ'בק" })).toBeInTheDocument();
});

it("toggles a body type filter", () => {
  const { onChange } = setup();
  fireEvent.click(screen.getByRole("button", { name: "סדאן" }));
  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({ bodyTypes: ["sedan"] }),
  );
});
```

- [ ] **Step 10: Run ListingsFilterBar tests to verify they fail**

Run: `cd web && npx vitest run src/components/ListingsFilterBar.test.tsx 2>&1 | tail -20`
Expected: FAIL — body type chips don't exist yet.

- [ ] **Step 11: Add body type chips to ListingsFilterBar**

In `web/src/components/ListingsFilterBar.tsx`, add a display label map at the top of the file (after the `toggleInArray` function):

```typescript
const bodyTypeLabels: Record<string, string> = {
  sedan: "סדאן",
  hatchback: "האצ'בק",
  suv: "SUV",
  wagon: "סטיישן",
  coupe: "קופה",
  convertible: "קבריולה",
  minivan: "מיניוון",
  pickup: "טנדר",
};
```

In the JSX, after the gearbox chips block (`{facets.gearboxes.map(...)}`), add:

```tsx
{facets.bodyTypes.map((bt) => (
  <Toggle
    key={bt}
    variant="outline"
    size="sm"
    pressed={filters.bodyTypes.includes(bt)}
    onPressedChange={() =>
      set({ bodyTypes: toggleInArray(filters.bodyTypes, bt) })
    }
  >
    {bodyTypeLabels[bt] ?? bt}
  </Toggle>
))}
```

- [ ] **Step 12: Run all frontend tests**

Run: `cd web && npx vitest run 2>&1 | tail -20`
Expected: all tests PASS.

- [ ] **Step 13: Run Go lint**

Run: `golangci-lint run ./...`
Expected: no errors.

- [ ] **Step 14: Commit**

```bash
git add internal/api/listings.go internal/api/bookmarks.go internal/api/instant_search.go \
  web/src/lib/api.ts web/src/hooks/useListingFilters.ts web/src/hooks/useListingFilters.test.ts \
  web/src/components/ListingsFilterBar.tsx web/src/components/ListingsFilterBar.test.tsx
git commit -m "feat: add body type filter chips to listings UI

Signed-off-by: Daniel Sionov <kingddd301@gmail.com>"
```
