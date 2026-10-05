export type BadgeVariant = "success" | "warning" | "danger" | "muted" | "http" | "socks5" | "default";

// latencyVariant — цвет бейджа по величине задержки.
export function latencyVariant(ms: number | null | undefined): BadgeVariant {
  if (ms == null) {
    return "muted";
  }
  if (ms < 150) {
    return "success";
  }
  if (ms <= 500) {
    return "warning";
  }
  return "danger";
}

export function formatLatency(ms: number | null | undefined): string {
  return ms == null ? "—" : `${ms} мс`;
}

export function protocolVariant(protocol: string): BadgeVariant {
  return protocol === "socks5" ? "socks5" : "http";
}

export function formatDownload(mbps: number | null | undefined): string {
  return mbps == null ? "—" : `${mbps.toFixed(1)} Мбит/с`;
}
