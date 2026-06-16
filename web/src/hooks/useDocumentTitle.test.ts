import { describe, it, expect, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useDocumentTitle, DEFAULT_DOCUMENT_TITLE } from "./useDocumentTitle";

describe("useDocumentTitle", () => {
  const original = document.title;
  afterEach(() => {
    document.title = original;
  });

  it("suffixes the brand onto a page title", () => {
    renderHook(() => useDocumentTitle("התחברות"));
    expect(document.title).toBe("התחברות · CarWatch");
  });

  it("applies the default brand title when given no argument", () => {
    renderHook(() => useDocumentTitle());
    expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
  });
});
