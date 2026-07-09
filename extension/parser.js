// Parses Yad2's gateway JSON feed
// (GET https://gw.yad2.co.il/feed-search-vehicles/cars?...).
//
// Yad2 retired the server-rendered __NEXT_DATA__ blob on the /vehicles/cars
// HTML route (it is now a Radware bot-challenge shell), so we consume the same
// JSON API the site's own SPA calls. The response groups ads into seller
// buckets; each item carries all fields we need INLINE (km, city, image,
// publish date) — no per-item enrichment required.

const BUCKET_KEYS = ["private", "commercial", "platinum", "boost", "solo"];
const COMMERCIAL_BUCKETS = new Set(["commercial", "platinum", "solo", "boost"]);

// Prefer the English label, fall back to Hebrew text. Handles Yad2's
// {id, text, text_eng} field shape (text_eng is frequently null).
function label(field) {
  if (!field) return "";
  return field.text_eng || field.textEng || field.text || "";
}

// The search's gearbox is stored as the Hebrew "אוטומט" and the backend filter
// (internal/filter/filter.go) matches on that exact string. The gw feed has no
// clean gearbox field, and the item detail's gearBox.text_eng is English
// ("Automatic") — using it made every enriched listing FAIL the gearbox filter
// and get dropped (so km never saved). Derive gearbox the same way the Go
// scraper does (parser.go subModelGearRe): the sub-model text carries an "אוט׳"
// marker for automatics. Non-automatics stay empty, which the filter skips.
const AUTO_GEARBOX_RE = /אוט[׳']/;
// Test the RAW Hebrew sub-model text (field.text), not label(): label() prefers
// text_eng, so it would miss the Hebrew "אוט׳" marker whenever English text is
// present and wrongly leave gearbox empty.
function deriveGearBox(subModelField) {
  const text = (subModelField && subModelField.text) || "";
  return AUTO_GEARBOX_RE.test(text) ? "אוטומט" : "";
}

function fieldId(field) {
  if (!field) return 0;
  const id = field.id;
  if (typeof id === "number") return id;
  if (typeof id === "string" && id.trim() !== "") {
    const n = Number(id);
    return Number.isFinite(n) ? n : 0;
  }
  return 0;
}

function resolveImage(item) {
  const md = item.meta_data || {};
  if (md.cover_image) return md.cover_image;
  if (Array.isArray(md.images) && md.images.length) return md.images[0];
  return "";
}

function itemToListing(item, isCommercial) {
  const token = item.token;
  if (!token) return null;

  const sf = item.search_fields || {};
  const md = item.meta_data || {};
  const dates = item.dates || {};

  const listing = {
    token,
    manufacturer: label(sf.manufacturer),
    manufacturer_id: fieldId(sf.manufacturer),
    model: label(sf.model),
    model_id: fieldId(sf.model),
    sub_model: label(sf.sub_model),
    sub_model_id: fieldId(sf.sub_model),
    year: sf.year || 0,
    price: item.price || 0,
    km: typeof sf.km === "number" ? sf.km : 0,
    hand: fieldId(sf.hand),
    city: label(item.address && item.address.city),
    area: label(item.address && item.address.area),
    image_url: resolveImage(item),
    page_link: `https://www.yad2.co.il/vehicles/item/${token}`,
    engine_volume: sf.engine_volume || 0,
    engine_type: label(sf.engine_type),
    gear_box: deriveGearBox(sf.sub_model),
    body_type: label(sf.body_type),
    description: (md.description || "").trim(),
    is_commercial: isCommercial,
  };

  // dates.start is the ORIGINAL publish date (unique per ad). dates.update /
  // dates.rebounce are the feed-wide "refreshed" timestamp — do NOT use them,
  // or every ad looks like it was posted today.
  if (dates.start) {
    listing.created_at = dates.start;
  }

  return listing;
}

// Accepts the raw gw response (object or JSON string). Buckets live under
// `.data`, but tolerate a top-level bucket object too.
function parseFeed(json) {
  let data;
  try {
    data = typeof json === "string" ? JSON.parse(json) : json;
  } catch {
    return { error: "invalid feed JSON" };
  }
  if (!data || typeof data !== "object") return { error: "empty feed" };

  const buckets = data.data && typeof data.data === "object" ? data.data : data;

  const listings = [];
  let found = false;
  for (const key of BUCKET_KEYS) {
    const items = buckets[key];
    if (!Array.isArray(items)) continue;
    found = true;
    const isCommercial = COMMERCIAL_BUCKETS.has(key);
    for (const item of items) {
      const listing = itemToListing(item, isCommercial);
      if (listing) listings.push(listing);
    }
  }

  if (!found) return { error: "no known buckets in feed" };

  const seen = new Set();
  const deduped = listings.filter((l) => {
    if (seen.has(l.token)) return false;
    seen.add(l.token);
    return true;
  });
  return { listings: deduped };
}

// Parses the per-item detail endpoint
// (GET https://gw.yad2.co.il/vehicles-item/{token}) into the fields the list
// feed omits — chiefly `km`, `city`, and `image_url`. The detail payload is
// camelCase (vs the feed's snake_case); `label()` already tolerates both.
// Returns only the fields that are present, so the caller can overlay them onto
// the feed listing without wiping known values with blanks.
function parseItemDetail(json) {
  let d;
  try {
    d = typeof json === "string" ? JSON.parse(json) : json;
  } catch {
    return { error: "invalid item JSON" };
  }
  if (!d || typeof d !== "object") return { error: "empty item" };
  d = d.data && typeof d.data === "object" ? d.data : d;
  if (!d.token) return { error: "item missing token" };

  const md = d.metaData || d.meta_data || {};
  const vd = d.vehicleDates || {};
  const dates = d.dates || {};
  const fields = { token: d.token };

  if (typeof d.km === "number") fields.km = d.km;
  const city = label(d.address && d.address.city);
  if (city) fields.city = city;
  const area = label(d.address && d.address.area);
  if (area) fields.area = area;
  const image = md.coverImage || md.cover_image || (Array.isArray(md.images) && md.images[0]);
  if (image) fields.image_url = image;
  if (vd.yearOfProduction) fields.year = vd.yearOfProduction;
  if (d.hand) fields.hand = fieldId(d.hand);
  if (typeof d.price === "number" && d.price > 0) fields.price = d.price;
  if (d.engineVolume) fields.engine_volume = d.engineVolume;
  const engineType = label(d.engineType);
  if (engineType) fields.engine_type = engineType;
  // Derive gearbox from the sub-model marker (Hebrew "אוטומט"), never the
  // item's English gearBox.text_eng — see deriveGearBox. Using English here
  // made enriched listings fail the backend gearbox filter and drop their km.
  const gearBox = deriveGearBox(d.subModel);
  if (gearBox) fields.gear_box = gearBox;
  const bodyType = label(d.bodyType);
  if (bodyType) fields.body_type = bodyType;
  const desc = (md.description || "").trim();
  if (desc) fields.description = desc;
  if (dates.createdAt) fields.created_at = dates.createdAt;

  return { fields };
}

// Export for Node-based tests; harmless in the service worker.
if (typeof module !== "undefined" && module.exports) {
  module.exports = { parseFeed, parseItemDetail, itemToListing };
}
