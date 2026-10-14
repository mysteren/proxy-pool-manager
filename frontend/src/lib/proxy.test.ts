import { describe, expect, it } from "vitest";

import { formatDownload, formatLatency, latencyVariant, protocolVariant } from "./proxy";

describe("latencyVariant", () => {
  it("не тестирован → muted", () => {
    expect(latencyVariant(null)).toBe("muted");
    expect(latencyVariant(undefined)).toBe("muted");
  });

  it("пороговые значения", () => {
    expect(latencyVariant(149)).toBe("success");
    expect(latencyVariant(150)).toBe("warning");
    expect(latencyVariant(500)).toBe("warning");
    expect(latencyVariant(501)).toBe("danger");
  });
});

describe("formatLatency", () => {
  it("пустое значение → прочерк", () => {
    expect(formatLatency(null)).toBe("—");
  });

  it("значение → миллисекунды", () => {
    expect(formatLatency(120)).toBe("120 мс");
  });
});

describe("protocolVariant", () => {
  it("socks5 и остальное", () => {
    expect(protocolVariant("socks5")).toBe("socks5");
    expect(protocolVariant("http")).toBe("http");
  });
});

describe("formatDownload", () => {
  it("пустое значение → прочерк", () => {
    expect(formatDownload(null)).toBe("—");
  });

  it("один знак после запятой", () => {
    expect(formatDownload(12.34)).toBe("12.3 Мбит/с");
  });
});
