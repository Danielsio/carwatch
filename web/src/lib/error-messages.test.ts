import { describe, it, expect } from "vitest";
import { errorToHebrew } from "./error-messages";
import { ApiError } from "./api";

describe("errorToHebrew", () => {
  it("returns auth-expired for 401", () => {
    const msg = errorToHebrew(new ApiError(401, "unauthorized"));
    expect(msg).toContain("הרשאה");
  });

  it("returns auth-expired for 403", () => {
    const msg = errorToHebrew(new ApiError(403, "forbidden"));
    expect(msg).toContain("הרשאה");
  });

  it("returns conflict message for 409", () => {
    const msg = errorToHebrew(new ApiError(409, ""));
    expect(msg).toContain("מתנגשת");
  });

  it("returns custom message for 409 with message", () => {
    const msg = errorToHebrew(new ApiError(409, "custom conflict"));
    expect(msg).toBe("custom conflict");
  });

  it("returns rate-limit message for 429", () => {
    const msg = errorToHebrew(new ApiError(429, "too many"));
    expect(msg).toContain("בקשות");
  });

  it("returns server error for 500+", () => {
    const msg = errorToHebrew(new ApiError(500, "internal"));
    expect(msg).toContain("שרת");
  });

  it("returns generic ApiError message for other status codes", () => {
    const msg = errorToHebrew(new ApiError(422, "custom error"));
    expect(msg).toBe("custom error");
  });

  it("returns network error for TypeError with 'failed to fetch'", () => {
    const msg = errorToHebrew(new TypeError("Failed to fetch"));
    expect(msg).toContain("חיבור");
  });

  it("returns network error for TypeError with 'network'", () => {
    const msg = errorToHebrew(new TypeError("network error"));
    expect(msg).toContain("חיבור");
  });

  it("returns generic error for unknown Error", () => {
    const msg = errorToHebrew(new Error("something unknown"));
    expect(msg).toContain("שגיאה");
  });

  it("returns generic error for non-Error value", () => {
    const msg = errorToHebrew("string error");
    expect(msg).toContain("שגיאה");
  });
});
