import { describe, it, expect, beforeEach } from "vitest";
import { getAuthToken, setAuthTokenGetter } from "./auth-token";

describe("auth-token", () => {
  beforeEach(() => {
    setAuthTokenGetter(async () => null);
  });

  it("returns null by default", async () => {
    const token = await getAuthToken();
    expect(token).toBeNull();
  });

  it("returns the token from a custom getter", async () => {
    setAuthTokenGetter(async () => "my-token");
    const token = await getAuthToken();
    expect(token).toBe("my-token");
  });

  it("passes forceRefresh to the getter", async () => {
    let receivedRefresh: boolean | undefined;
    setAuthTokenGetter(async (forceRefresh) => {
      receivedRefresh = forceRefresh;
      return "token";
    });
    await getAuthToken(true);
    expect(receivedRefresh).toBe(true);
  });
});
