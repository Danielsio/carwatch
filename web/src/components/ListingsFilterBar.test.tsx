import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ListingsFilterBar } from "./ListingsFilterBar";
import { EMPTY_FILTERS, type ListingFacets } from "@/hooks/useListingFilters";

const facets: ListingFacets = {
  fuels: ["בנזין", "היברידי"],
  gearboxes: ["אוטומטית"],
  bodyTypes: ["sedan", "hatchback"],
  priceMax: 200000,
  yearMin: 2015,
  yearMax: 2023,
  kmMax: 150000,
};

function setup(overrides = {}) {
  const onChange = vi.fn();
  const onClear = vi.fn();
  render(
    <ListingsFilterBar
      filters={EMPTY_FILTERS}
      onChange={onChange}
      onClear={onClear}
      facets={facets}
      resultCount={10}
      totalCount={42}
      {...overrides}
    />,
  );
  return { onChange, onClear };
}

describe("ListingsFilterBar", () => {
  it("renders quick toggles and derived facet chips", () => {
    setup();
    expect(screen.getByRole("button", { name: /מציאות/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /חדשות/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "בנזין" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "היברידי" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "אוטומטית" })).toBeInTheDocument();
  });

  it("toggles a quick filter", () => {
    const { onChange } = setup();
    fireEvent.click(screen.getByRole("button", { name: /מציאות/ }));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ dealsOnly: true }),
    );
  });

  it("adds a fuel facet to the array", () => {
    const { onChange } = setup();
    fireEvent.click(screen.getByRole("button", { name: "היברידי" }));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ fuels: ["היברידי"] }),
    );
  });

  it("renders body type chips with Hebrew labels", () => {
    setup();
    expect(screen.getByRole("button", { name: "סדאן" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "האצ'בק" })).toBeInTheDocument();
  });

  it("toggles a body type filter", () => {
    const { onChange } = setup();
    fireEvent.click(screen.getByRole("button", { name: "סדאן" }));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ bodyTypes: ["sedan"] }),
    );
  });

  it("emits text changes", () => {
    const { onChange } = setup();
    fireEvent.change(screen.getByRole("searchbox"), {
      target: { value: "mazda" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ text: "mazda" }),
    );
  });

  it("reveals range sliders when expanded", () => {
    setup();
    expect(screen.queryByLabelText("מחיר מקסימלי")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /טווחים/ }));
    expect(screen.getByLabelText("מחיר מקסימלי")).toBeInTheDocument();
    expect(screen.getByLabelText("שנה מינימלית")).toBeInTheDocument();
    expect(screen.getByLabelText("קילומטראז׳ מקסימלי")).toBeInTheDocument();
  });

  it("shows count + clear only when filters are active", () => {
    const { onClear } = setup({ filters: { ...EMPTY_FILTERS, dealsOnly: true } });
    const clear = screen.getByRole("button", { name: /נקה/ });
    expect(clear).toBeInTheDocument();
    expect(screen.getByText(/מתוך/)).toBeInTheDocument();
    fireEvent.click(clear);
    expect(onClear).toHaveBeenCalled();
  });

  it("hides count + clear when no filters are active", () => {
    setup();
    expect(screen.queryByRole("button", { name: /נקה/ })).not.toBeInTheDocument();
  });
});
