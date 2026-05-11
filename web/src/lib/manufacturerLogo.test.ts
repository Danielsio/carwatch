import { describe, expect, it } from "vitest";
import { manufacturerLogoSrc } from "./manufacturerLogo";

describe("manufacturerLogoSrc", () => {
  it("returns public path when a bundled mark exists", () => {
    expect(manufacturerLogoSrc("Mazda")).toBe("/manufacturers/mazda.svg");
    expect(manufacturerLogoSrc("  Toyota ")).toBe("/manufacturers/toyota.svg");
  });

  it("maps DS catalog name to dsautomobiles asset", () => {
    expect(manufacturerLogoSrc("DS")).toBe("/manufacturers/dsautomobiles.svg");
  });

  it("returns null when no SVG is bundled for the brand", () => {
    expect(manufacturerLogoSrc("BYD")).toBeNull();
    expect(manufacturerLogoSrc("")).toBeNull();
  });
});
