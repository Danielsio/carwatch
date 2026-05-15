import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock auth-token module before importing api
const mockGetAuthToken = vi.fn();
vi.mock("@/lib/auth-token", () => ({
  getAuthToken: (...args: unknown[]) => mockGetAuthToken(...args),
}));

describe("fetchAPI (via api.listing)", () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    mockGetAuthToken.mockReset().mockResolvedValue("test-token");
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  // Dynamic import so mocks are in place
  async function getApi() {
    const mod = await import("@/lib/api");
    return mod;
  }

  it("returns parsed JSON on successful fetch", async () => {
    const payload = { token: "abc", manufacturer: "Toyota", model: "Corolla", year: 2022, price: 80000, km: 30000, hand: 1, city: "Haifa", page_link: "https://example.com/abc", first_seen_at: "2025-01-01T00:00:00Z" };
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve(payload),
    });

    const { api } = await getApi();
    const result = await api.listing("abc");

    expect(result).toEqual(payload);
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    const [url, opts] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe("/api/v1/listings/abc");
    expect(opts.headers.get("Authorization")).toBe("Bearer test-token");
  });

  it("retries with fresh token on 401 and succeeds", async () => {
    const errorResp = {
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: "unauthorized" }),
    };
    const successResp = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ token: "abc" }),
    };

    (globalThis.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(errorResp)
      .mockResolvedValueOnce(successResp);

    mockGetAuthToken
      .mockResolvedValueOnce("old-token")    // initial call
      .mockResolvedValueOnce("fresh-token");  // force-refresh call

    const { api } = await getApi();
    const result = await api.listing("abc");

    expect(result).toEqual({ token: "abc" });
    expect(globalThis.fetch).toHaveBeenCalledTimes(2);
    // The second fetch should use the fresh token
    const [, retryOpts] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[1];
    expect(retryOpts.headers.get("Authorization")).toBe("Bearer fresh-token");
  });

  it("throws ApiError on 401 when no fresh token is available", async () => {
    const errorBody = { error: "unauthorized" };
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: () => Promise.resolve(errorBody),
    });

    mockGetAuthToken
      .mockResolvedValueOnce("old-token")
      .mockResolvedValueOnce(null); // no fresh token

    const { api, ApiError } = await getApi();

    try {
      await api.listing("abc");
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(401);
    }
  });

  it("throws ApiError with status code on 4xx/5xx", async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: "forbidden" }),
    });

    const { api, ApiError } = await getApi();

    try {
      await api.listing("abc");
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(403);
      expect((err as InstanceType<typeof ApiError>).message).toBe("forbidden");
    }
  });

  it("throws on network error", async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new TypeError("Failed to fetch"),
    );

    const { api } = await getApi();
    await expect(api.listing("abc")).rejects.toThrow("Failed to fetch");
  });
});
