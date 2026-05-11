import { describe, expect, it } from "vitest";
import {
  manufacturerLogoSrc,
  manufacturerLogoSrcFromCatalogId,
} from "./manufacturerLogo";

function expectedManufacturerLogo(slug: string): string {
  let base = import.meta.env.BASE_URL;
  if (!base) base = "/";
  if (!base.endsWith("/")) base = `${base}/`;
  return `${base}manufacturers/${slug}.svg`;
}

describe("manufacturerLogoSrc", () => {
  it("returns public path when a bundled mark exists", () => {
    expect(manufacturerLogoSrc("Mazda")).toBe(expectedManufacturerLogo("mazda"));
    expect(manufacturerLogoSrc("  Toyota ")).toBe(
      expectedManufacturerLogo("toyota"),
    );
    expect(manufacturerLogoSrc("Citroën")).toBe(expectedManufacturerLogo("citroen"));
    expect(manufacturerLogoSrc("Land Rover")).toBe(
      expectedManufacturerLogo("landrover"),
    );
  });

  it("resolves Hebrew catalog display names (Yad2-primary catalog)", () => {
    expect(manufacturerLogoSrc("טויוטה")).toBe(expectedManufacturerLogo("toyota"));
    expect(manufacturerLogoSrc("מאזדה")).toBe(expectedManufacturerLogo("mazda"));
  });

  it("maps DS catalog name to dsautomobiles asset", () => {
    expect(manufacturerLogoSrc("DS")).toBe(expectedManufacturerLogo("dsautomobiles"));
  });

  it("returns null when no SVG is bundled for the brand", () => {
    expect(manufacturerLogoSrc("BYD")).toBeNull();
    expect(manufacturerLogoSrc("")).toBeNull();
  });
});

describe("manufacturerLogoSrcFromCatalogId", () => {
  it("resolves Toyota and Mazda by Yad2 manufacturer id", () => {
    expect(manufacturerLogoSrcFromCatalogId(19)).toBe(
      expectedManufacturerLogo("toyota"),
    );
    expect(manufacturerLogoSrcFromCatalogId(27)).toBe(
      expectedManufacturerLogo("mazda"),
    );
  });

  it("returns null for unknown or invalid ids", () => {
    expect(manufacturerLogoSrcFromCatalogId(99999)).toBeNull();
    expect(manufacturerLogoSrcFromCatalogId(0)).toBeNull();
  });
});
