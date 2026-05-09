import { describe, expect, it, vi } from "vitest";
import { feedSSE } from "./sse-parse";

describe("feedSSE", () => {
  it("ignores comment-only blocks (keepalive)", () => {
    const onData = vi.fn();
    const rest = feedSSE(": keepalive\n\n", "", onData);
    expect(rest).toBe("");
    expect(onData).not.toHaveBeenCalled();
  });

  it("parses a single data line", () => {
    const onData = vi.fn();
    feedSSE('data: {"a":1}\n\n', "", onData);
    expect(onData).toHaveBeenCalledTimes(1);
    expect(onData).toHaveBeenCalledWith('{"a":1}');
  });

  it("joins multi-line data fields per SSE spec", () => {
    const onData = vi.fn();
    let buf = feedSSE("data: hello\n", "", onData);
    expect(onData).not.toHaveBeenCalled();
    buf = feedSSE("data: world\n\n", buf, onData);
    expect(onData).toHaveBeenCalledWith("hello\nworld");
    expect(buf).toBe("");
  });

  it("buffers incomplete events", () => {
    const onData = vi.fn();
    let buf = feedSSE('data: {"x":', "", onData);
    expect(onData).not.toHaveBeenCalled();
    buf = feedSSE("1}\n", buf, onData);
    expect(onData).not.toHaveBeenCalled();
    buf = feedSSE("\n", buf, onData);
    expect(onData).toHaveBeenCalledTimes(1);
    expect(onData).toHaveBeenCalledWith('{"x":1}');
    expect(buf).toBe("");
  });

  it("handles multiple events in one chunk", () => {
    const onData = vi.fn();
    feedSSE('data: a\n\ndata: b\n\n', "", onData);
    expect(onData).toHaveBeenNthCalledWith(1, "a");
    expect(onData).toHaveBeenNthCalledWith(2, "b");
  });
});
