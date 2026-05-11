import { describe, expect, it } from "vitest";
import { manufacturerLogoSrc } from "./manufacturerLogo";

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

  it("maps DS catalog name to dsautomobiles asset", () => {
    expect(manufacturerLogoSrc("DS")).toBe(expectedManufacturerLogo("dsautomobiles"));
  });

  it("returns null when no SVG is bundled for the brand", () => {
    expect(manufacturerLogoSrc("BYD")).toBeNull();
    expect(manufacturerLogoSrc("")).toBeNull();
  });
});
