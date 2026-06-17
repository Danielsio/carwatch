import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { Combobox, type ComboboxOption } from "./Combobox";

const OPTIONS: ComboboxOption[] = [
  { value: 0, label: "כל היצרנים" },
  { value: 1, label: "טויוטה (Toyota)", keywords: ["Toyota", "טויוטה"] },
  { value: 2, label: "לנד רובר (Land Rover)", keywords: ["Land Rover", "לנד רובר"] },
  { value: 3, label: "מרצדס (Mercedes)", keywords: ["Mercedes", "מרצדס"] },
];

function Harness({ onChange }: { onChange?: (v: ComboboxOption["value"]) => void }) {
  const [value, setValue] = useState<ComboboxOption["value"]>(0);
  return (
    <Combobox
      options={OPTIONS}
      value={value}
      onChange={(v) => {
        setValue(v);
        onChange?.(v);
      }}
      placeholder="כל היצרנים"
      searchPlaceholder="חיפוש יצרן…"
      emptyText="לא נמצא יצרן"
    />
  );
}

describe("Combobox", () => {
  it("shows the selected option's label in the trigger", () => {
    render(
      <Combobox options={OPTIONS} value={2} onChange={() => {}} />,
    );
    expect(screen.getByText("לנד רובר (Land Rover)")).toBeInTheDocument();
  });

  it("opens on click and lists all options", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.click(screen.getByRole("button"));
    await screen.findByPlaceholderText("חיפוש יצרן…");

    expect(screen.getAllByRole("option")).toHaveLength(OPTIONS.length);
  });

  it("filters options through the trie as the user types (prefix of any word)", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.click(screen.getByRole("button"));
    const input = await screen.findByPlaceholderText("חיפוש יצרן…");

    // "rover" is the second word of "Land Rover"
    await user.type(input, "rover");

    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options).toHaveLength(1);
      expect(options[0]).toHaveTextContent("לנד רובר");
    });
  });

  it("matches a Hebrew prefix", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.click(screen.getByRole("button"));
    const input = await screen.findByPlaceholderText("חיפוש יצרן…");
    await user.type(input, "מרצ");

    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options).toHaveLength(1);
      expect(options[0]).toHaveTextContent("מרצדס");
    });
  });

  it("shows the empty message when nothing matches", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.click(screen.getByRole("button"));
    const input = await screen.findByPlaceholderText("חיפוש יצרן…");
    await user.type(input, "zzz");

    expect(await screen.findByText("לא נמצא יצרן")).toBeInTheDocument();
  });

  it("calls onChange and reflects the selection when an option is picked", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await user.click(screen.getByRole("button"));
    const input = await screen.findByPlaceholderText("חיפוש יצרן…");
    await user.type(input, "toy");

    await user.click(await screen.findByRole("option"));

    expect(onChange).toHaveBeenCalledWith(1);
    // trigger now shows the picked label
    expect(screen.getByText("טויוטה (Toyota)")).toBeInTheDocument();
  });

  it("is disabled when the disabled prop is set", () => {
    render(<Combobox options={OPTIONS} value={0} onChange={() => {}} disabled />);
    expect(screen.getByRole("button")).toBeDisabled();
  });
});
