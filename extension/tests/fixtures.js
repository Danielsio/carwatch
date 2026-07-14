// Fixtures shaped like the real gw.yad2.co.il responses.
//
// Refresh these whenever Yad2 changes its payload: capture a live response from
// the DevTools network tab of a yad2 vehicles search (feed) and an item page
// (item), trim to a couple of entries, and drop it in here. If Yad2's shape
// changes and these are NOT refreshed, the tests keep passing while production
// breaks — so treat a parser bug in the wild as a missing fixture.

// GET https://gw.yad2.co.il/feed-search-vehicles/cars?...
// Ads are grouped into seller buckets; private vs commercial matters for the
// seller filter. Note `search_fields` is snake_case here (the item endpoint is
// camelCase) and that most feed rows carry no km.
const feedResponse = {
  data: {
    private: [
      {
        token: "priv-1",
        price: 82000,
        search_fields: {
          manufacturer: { id: 27, text: "מאזדה", text_eng: "Mazda" },
          model: { id: 10514, text: "3", text_eng: "3" },
          sub_model: { id: 5, text: "1.5 אוט׳ Active" },
          year: 2019,
          hand: { id: 2, text: "יד שנייה" },
          km: 64000,
          engine_volume: 1500,
          engine_type: { id: 1, text: "בנזין", text_eng: "Petrol" },
          body_type: { id: 3, text: "סדאן", text_eng: "Sedan" },
        },
        address: { city: { text: "תל אביב" }, area: { text: "תל אביב והמרכז" } },
        meta_data: { cover_image: "https://img.yad2.co.il/priv-1.jpg", description: "  רכב שמור  " },
        dates: { start: "2026-07-01T09:00:00", update: "2026-07-14T06:00:00" },
      },
      {
        // No km, no image: the common case the item-detail enrichment exists for.
        token: "priv-2",
        price: 0, // seller did not publish a price
        search_fields: {
          manufacturer: { id: 27, text: "מאזדה", text_eng: "Mazda" },
          model: { id: 10514, text: "3" },
          sub_model: { id: 6, text: "1.6 ידני" }, // manual: no "אוט׳" marker
          year: 2018,
          hand: { id: "3" }, // ids sometimes arrive as strings
        },
        address: { city: { text: "חיפה" } },
        meta_data: {},
        dates: { start: "2026-06-20T11:30:00" },
      },
    ],
    commercial: [
      {
        token: "comm-1",
        price: 91000,
        search_fields: {
          manufacturer: { id: 27, text: "מאזדה", text_eng: "Mazda" },
          model: { id: 10514, text: "3" },
          sub_model: { id: 7, text: "2.0 אוט׳ Premium" },
          year: 2020,
          km: 30000,
        },
        address: { city: { text: "ירושלים" } },
        meta_data: { images: ["https://img.yad2.co.il/comm-1.jpg"] },
        dates: { start: "2026-07-10T08:00:00" },
      },
    ],
  },
};

// GET https://gw.yad2.co.il/vehicles-item/{token} — camelCase, and the source
// of km/city/image for feed rows that lack them.
const itemResponse = {
  data: {
    token: "priv-2",
    price: 74000,
    km: 88000,
    hand: { id: 3 },
    subModel: { id: 6, text: "1.6 אוט׳ Comfort" },
    engineVolume: 1600,
    engineType: { id: 1, text: "בנזין", text_eng: "Petrol" },
    bodyType: { id: 3, text: "סדאן", text_eng: "Sedan" },
    vehicleDates: { yearOfProduction: 2018 },
    address: { city: { text: "חיפה" }, area: { text: "חיפה והצפון" } },
    metaData: { coverImage: "https://img.yad2.co.il/priv-2.jpg", description: "יד שלישית" },
    dates: { createdAt: "2026-06-20T11:30:00" },
  },
};

// What Radware serves when the tab's clearance has lapsed: an HTML shell, not
// JSON — and often with a 200 status, which is why the body must be sniffed.
const radwareChallengeBody =
  '<html><head><meta http-equiv="refresh" content="0"></head><body>Verifying...</body></html>';

module.exports = { feedResponse, itemResponse, radwareChallengeBody };
