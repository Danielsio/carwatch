const BUCKET_KEYS = ["private", "commercial", "platinum", "boost", "solo"];

const COMMERCIAL_BUCKETS = new Set(["commercial", "platinum", "solo", "boost"]);

function textFromField(field) {
  if (!field) return "";
  return field.english_text || field.textEng || field.text || "";
}

function parseHand(raw) {
  if (typeof raw === "number") return raw;
  if (raw && typeof raw === "object" && raw.id) return raw.id;
  return 0;
}

function resolveImageURL(item) {
  const md = item.metaData || {};
  return (
    md.coverImage ||
    md.cover_image ||
    item.coverImage ||
    item.cover_image ||
    (md.images && md.images[0]) ||
    (item.images && item.images[0]) ||
    ""
  );
}

function itemToListing(item, isCommercial) {
  const token = item.token;
  if (!token) return null;

  const year =
    item.year_of_production ||
    (item.vehicleDates && item.vehicleDates.yearOfProduction) ||
    0;

  const listing = {
    token,
    manufacturer: textFromField(item.manufacturer),
    manufacturer_id: item.manufacturer?.id || 0,
    model: textFromField(item.model),
    model_id: item.model?.id || 0,
    sub_model: textFromField(item.subModel),
    sub_model_id: item.subModel?.id || 0,
    year,
    price: item.price || 0,
    km: item.km || item.kilometers || 0,
    hand: parseHand(item.hand),
    city: textFromField(item.address?.city),
    area: textFromField(item.address?.area),
    image_url: resolveImageURL(item),
    page_link: `https://www.yad2.co.il/vehicles/item/${token}`,
    engine_volume: item.engine_volume || item.engineVolume || 0,
    engine_type: textFromField(item.engineType),
    gear_box: textFromField(item.gearBox),
    body_type: textFromField(item.bodyType),
    description: item.metaData?.description || "",
    is_commercial: isCommercial,
  };

  if (item.dates?.createdAt || item.createdAt) {
    listing.created_at = item.dates?.createdAt || item.createdAt;
  }

  return listing;
}

function parseNextData(html) {
  const match = html.match(
    /<script[^>]*id=["']__NEXT_DATA__["'][^>]*>([\s\S]*?)<\/script>/i,
  );
  if (!match || !match[1]) return { error: "no __NEXT_DATA__" };

  let data;
  try {
    data = JSON.parse(match[1]);
  } catch {
    return { error: "invalid __NEXT_DATA__ JSON" };
  }

  const queries =
    data?.props?.pageProps?.dehydratedState?.queries || [];

  for (const q of queries) {
    const stateData = q?.state?.data;
    if (!stateData) continue;

    const listings = [];
    let found = false;

    for (const key of BUCKET_KEYS) {
      const items = stateData[key];
      if (!Array.isArray(items)) continue;
      found = true;
      const isCommercial = COMMERCIAL_BUCKETS.has(key);
      for (const item of items) {
        const listing = itemToListing(item, isCommercial);
        if (listing) listings.push(listing);
      }
    }

    if (found) {
      const seen = new Set();
      const deduped = listings.filter((l) => {
        if (seen.has(l.token)) return false;
        seen.add(l.token);
        return true;
      });
      return { listings: deduped };
    }

    // Legacy format: data.feed.feed_items
    const feedItems = stateData?.data?.feed?.feed_items;
    if (Array.isArray(feedItems)) {
      const listings = feedItems
        .map((item) => itemToListing(item, null))
        .filter(Boolean);
      return { listings };
    }
  }

  return { error: "no feed items found" };
}
