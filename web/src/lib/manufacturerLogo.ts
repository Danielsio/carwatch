/**
 * Map manufacturer identity to bundled SVGs in /public/manufacturers.
 * Icons are from Simple Icons (CC0) — see public/manufacturers/README.txt.
 *
 * Yad2 often seeds the server catalog with Hebrew as the display `Name`, so
 * ASCII-only slugging is not enough. We match:
 * - catalog `manufacturer_id` (stable), and
 * - Hebrew names from internal/catalog/catalog_data.json `name_he`
 *   (regenerate maps when that file changes).
 */

/** Slugs that exist under web/public/manufacturers/*.svg */
const MANUFACTURER_LOGO_SLUGS = new Set([
  "acura",
  "alfaromeo",
  "astonmartin",
  "audi",
  "bentley",
  "bmw",
  "cadillac",
  "chevrolet",
  "chrysler",
  "citroen",
  "dacia",
  "dsautomobiles",
  "ferrari",
  "fiat",
  "ford",
  "honda",
  "hyundai",
  "infiniti",
  "jaguar",
  "jeep",
  "kia",
  "lada",
  "lamborghini",
  "landrover",
  "maserati",
  "mazda",
  "mclaren",
  "mercedes",
  "mg",
  "mini",
  "mitsubishi",
  "nissan",
  "opel",
  "peugeot",
  "polestar",
  "porsche",
  "ram",
  "renault",
  "rollsroyce",
  "seat",
  "skoda",
  "smart",
  "subaru",
  "suzuki",
  "tesla",
  "toyota",
  "volkswagen",
  "volvo",
]);

/** When English catalog `name` does not produce the Simple Icons file slug. */
const SLUG_OVERRIDES: Record<string, string> = {
  ds: "dsautomobiles",
};

/** Catalog manufacturer id → logo file slug (see catalog_data.json). */
const MANUFACTURER_ID_TO_LOGO_SLUG: Record<number, string> = {
  1: "audi",
  2: "opel",
  3: "infiniti",
  5: "alfaromeo",
  6: "mg",
  7: "bmw",
  10: "jeep",
  12: "dacia",
  14: "dsautomobiles",
  17: "honda",
  18: "volvo",
  19: "toyota",
  20: "jaguar",
  21: "hyundai",
  24: "landrover",
  27: "mazda",
  28: "maserati",
  29: "mini",
  30: "mitsubishi",
  31: "mercedes",
  32: "nissan",
  35: "subaru",
  36: "suzuki",
  37: "seat",
  38: "citroen",
  39: "smart",
  40: "skoda",
  41: "volkswagen",
  43: "ford",
  44: "porsche",
  45: "fiat",
  46: "peugeot",
  47: "cadillac",
  48: "kia",
  49: "chrysler",
  51: "renault",
  52: "chevrolet",
  54: "astonmartin",
  55: "bentley",
  57: "ferrari",
  58: "rollsroyce",
  62: "tesla",
  63: "lamborghini",
  73: "mclaren",
  80: "lada",
  91: "ram",
  111: "acura",
  231: "polestar",
};

/** catalog_data.json `name_he` → logo file slug */
const HEBREW_MANUFACTURER_TO_LOGO_SLUG: Record<string, string> = {
  אקורה: "acura",
  "אלפא רומיאו": "alfaromeo",
  "אסטון מרטין": "astonmartin",
  אאודי: "audi",
  בנטלי: "bentley",
  "ב מ וו": "bmw",
  קאדילק: "cadillac",
  שברולט: "chevrolet",
  קרייזלר: "chrysler",
  סיטרואן: "citroen",
  "דאצ'יה": "dacia",
  "די.אס": "dsautomobiles",
  פרארי: "ferrari",
  פיאט: "fiat",
  פורד: "ford",
  הונדה: "honda",
  יונדאי: "hyundai",
  אינפיניטי: "infiniti",
  יגואר: "jaguar",
  "ג'יפ": "jeep",
  קיה: "kia",
  לאדה: "lada",
  למבורגיני: "lamborghini",
  "לנד רובר": "landrover",
  מזראטי: "maserati",
  מאזדה: "mazda",
  מקלארן: "mclaren",
  מרצדס: "mercedes",
  "אם ג'י": "mg",
  מיני: "mini",
  מיצובישי: "mitsubishi",
  ניסאן: "nissan",
  אופל: "opel",
  "פיג'ו": "peugeot",
  פולסטאר: "polestar",
  פורשה: "porsche",
  ראם: "ram",
  רנו: "renault",
  "רולס רויס": "rollsroyce",
  סיאט: "seat",
  סקודה: "skoda",
  סמארט: "smart",
  סובארו: "subaru",
  סוזוקי: "suzuki",
  טסלה: "tesla",
  טויוטה: "toyota",
  פולקסווגן: "volkswagen",
  וולוו: "volvo",
};

function siSlug(title: string): string {
  return title
    .normalize("NFKD")
    .replace(/\p{M}/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}

/** Vite base path + relative public file (no leading slash on rel). */
function publicAssetUrl(relPath: string): string {
  let base = import.meta.env.BASE_URL;
  if (!base) base = "/";
  if (!base.endsWith("/")) base = `${base}/`;
  const clean = relPath.replace(/^\/+/, "");
  return `${base}${clean}`;
}

function urlForSlug(slug: string): string | null {
  if (!MANUFACTURER_LOGO_SLUGS.has(slug)) return null;
  return publicAssetUrl(`manufacturers/${slug}.svg`);
}

/**
 * Logo URL from Yad2/catalog manufacturer id (preferred for search cards).
 */
export function manufacturerLogoSrcFromCatalogId(
  manufacturerId: number,
): string | null {
  if (manufacturerId <= 0) return null;
  const slug = MANUFACTURER_ID_TO_LOGO_SLUG[manufacturerId];
  if (!slug) return null;
  return urlForSlug(slug);
}

/**
 * Public URL for the manufacturer mark from display name (English or Hebrew), or null.
 */
export function manufacturerLogoSrc(manufacturerName: string): string | null {
  const raw = manufacturerName.trim();
  if (!raw) return null;
  const key = raw.toLowerCase();

  const fromOverride = SLUG_OVERRIDES[key];
  if (fromOverride) return urlForSlug(fromOverride);

  const fromHebrew = HEBREW_MANUFACTURER_TO_LOGO_SLUG[raw];
  if (fromHebrew) return urlForSlug(fromHebrew);

  const fromAscii = siSlug(raw);
  if (fromAscii && MANUFACTURER_LOGO_SLUGS.has(fromAscii)) {
    return publicAssetUrl(`manufacturers/${fromAscii}.svg`);
  }

  return null;
}
