// countryFlag превращает код страны (US, DE, …) в эмодзи-флаг.
export function countryFlag(code: string | null | undefined): string {
  if (!code || code.length !== 2) {
    return "";
  }
  const upper = code.toUpperCase();
  return String.fromCodePoint(
    ...[...upper].map((ch) => 0x1f1e6 + ch.charCodeAt(0) - 65),
  );
}

let regionNames: Intl.DisplayNames | null = null;
if (typeof Intl !== "undefined" && "DisplayNames" in Intl) {
  try {
    regionNames = new Intl.DisplayNames(["ru"], { type: "region" });
  } catch {
    regionNames = null;
  }
}

export function countryName(code: string | null | undefined): string {
  if (!code) {
    return "—";
  }
  const upper = code.toUpperCase();
  if (regionNames) {
    try {
      return regionNames.of(upper) ?? upper;
    } catch {
      return upper;
    }
  }
  return upper;
}

// haversineKm — расстояние между двумя точками по большому кругу.
export function haversineKm(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const r = 6371;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2;
  return 2 * r * Math.asin(Math.sqrt(a));
}

const numberFormat = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 });

export function formatDistance(km: number | null): string {
  if (km == null || Number.isNaN(km)) {
    return "—";
  }
  if (km < 1) {
    return "< 1 км";
  }
  return `${numberFormat.format(km)} км`;
}
