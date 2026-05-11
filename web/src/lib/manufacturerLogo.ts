/**
 * Map listing/catalog English manufacturer names to bundled SVGs in /public/manufacturers.
 * Icons are from Simple Icons (CC0) — see public/manufacturers/README.txt.
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

/** When catalog name does not produce the Simple Icons file slug. */
const SLUG_OVERRIDES: Record<string, string> = {
  ds: "dsautomobiles",
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

/**
 * Public URL for the manufacturer mark, or null if none is bundled.
 */
export function manufacturerLogoSrc(manufacturerName: string): string | null {
  const raw = manufacturerName.trim();
  if (!raw) return null;
  const key = raw.toLowerCase();
  const slug = SLUG_OVERRIDES[key] ?? siSlug(raw);
  if (!MANUFACTURER_LOGO_SLUGS.has(slug)) return null;
  return publicAssetUrl(`manufacturers/${slug}.svg`);
}
