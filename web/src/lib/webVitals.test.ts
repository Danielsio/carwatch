import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./auth-token", () => ({
  getAuthToken: vi.fn(),
}));

import { getAuthToken } from "./auth-token";
import { sendVitalsToServer } from "./webVitals";

const mockedGetAuthToken = vi.mocked(getAuthToken);

describe("sendVitalsToServer", () => {
  beforeEach(() => {
    mockedGetAuthToken.mockReset();
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response(null))));
  });

  it("sends an authorized vitals payload when a token exists", async () => {
    mockedGetAuthToken.mockResolvedValue("firebase-token");

    await sendVitalsToServer({
      name: "LCP",
      value: 1200,
      rating: "good",
      delta: 10,
      id: "vital-1",
      navigationType: "navigate",
    } as never);

    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/vitals",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer firebase-token",
          "Content-Type": "application/json",
        }),
      }),
    );
  });

  it("skips sending vitals when no token exists", async () => {
    mockedGetAuthToken.mockResolvedValue(null);

    await sendVitalsToServer({
      name: "TTFB",
      value: 100,
      rating: "good",
      delta: 3,
      id: "vital-2",
      navigationType: "reload",
    } as never);

    expect(fetch).not.toHaveBeenCalled();
  });
});
