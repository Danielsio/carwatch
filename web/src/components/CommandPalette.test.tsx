import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LayoutDashboard, Bookmark, Car } from "lucide-react";
import { CommandPalette, type CommandItem } from "./CommandPalette";

function makeCommands(): { commands: CommandItem[]; spies: ReturnType<typeof vi.fn>[] } {
  const spies = [vi.fn(), vi.fn(), vi.fn()];
  const commands: CommandItem[] = [
    { id: "a", label: "לוח בקרה", group: "ניווט", icon: LayoutDashboard, perform: spies[0] },
    { id: "b", label: "מועדפים", group: "ניווט", icon: Bookmark, perform: spies[1] },
    { id: "c", label: "מאזדה 3", group: "החיפושים שלי", icon: Car, keywords: "mazda", perform: spies[2] },
  ];
  return { commands, spies };
}

describe("CommandPalette", () => {
  it("renders nothing when closed", () => {
    const { commands } = makeCommands();
    render(<CommandPalette open={false} onClose={() => {}} commands={commands} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders a modal dialog with all commands when open", () => {
    const { commands } = makeCommands();
    render(<CommandPalette open onClose={() => {}} commands={commands} />);
    expect(screen.getByRole("dialog")).toHaveAttribute("aria-modal", "true");
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("filters commands by query (label and keywords)", () => {
    const { commands } = makeCommands();
    render(<CommandPalette open onClose={() => {}} commands={commands} />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "mazda" } });
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(1);
    expect(options[0]).toHaveTextContent("מאזדה 3");
  });

  it("shows an empty message when nothing matches", () => {
    const { commands } = makeCommands();
    render(<CommandPalette open onClose={() => {}} commands={commands} />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "zzzzz" } });
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    expect(screen.getByText("לא נמצאו תוצאות")).toBeInTheDocument();
  });

  it("moves the highlight with ArrowDown", () => {
    const { commands } = makeCommands();
    render(<CommandPalette open onClose={() => {}} commands={commands} />);
    const input = screen.getByRole("combobox");
    const options = screen.getAllByRole("option");
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(screen.getAllByRole("option")[1]).toHaveAttribute("aria-selected", "true");
  });

  it("runs the active command and closes on Enter", () => {
    const { commands, spies } = makeCommands();
    const onClose = vi.fn();
    render(<CommandPalette open onClose={onClose} commands={commands} />);
    const input = screen.getByRole("combobox");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(spies[1]).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("runs a command on click", () => {
    const { commands, spies } = makeCommands();
    const onClose = vi.fn();
    render(<CommandPalette open onClose={onClose} commands={commands} />);
    fireEvent.click(screen.getByText("מאזדה 3"));
    expect(spies[2]).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on Escape", () => {
    const { commands } = makeCommands();
    const onClose = vi.fn();
    render(<CommandPalette open onClose={onClose} commands={commands} />);
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes when the backdrop is clicked", () => {
    const { commands } = makeCommands();
    const onClose = vi.fn();
    const { container } = render(
      <CommandPalette open onClose={onClose} commands={commands} />,
    );
    const backdrop = container.querySelector('[aria-hidden="true"]') as HTMLElement;
    fireEvent.click(backdrop);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
