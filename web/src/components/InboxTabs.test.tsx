import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { InboxTabs } from "./InboxTabs";

vi.mock("@/hooks/useNotifications", () => ({
  useNotificationCount: () => ({ data: { count: 3 } }),
}));

function renderAt(path: string, action?: React.ReactNode) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <InboxTabs action={action} />
    </MemoryRouter>,
  );
}

describe("InboxTabs", () => {
  it("renders the three inbox tabs", () => {
    renderAt("/saved");
    expect(screen.getByRole("link", { name: /התראות/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /מועדפים/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /היסטוריה/ })).toBeInTheDocument();
  });

  it("marks the current route's tab as the active page", () => {
    renderAt("/history");
    expect(screen.getByRole("link", { name: /היסטוריה/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: /מועדפים/ })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("shows the unread badge on the notifications tab", () => {
    renderAt("/notifications");
    expect(
      screen.getByRole("link", { name: /התראות/ }),
    ).toHaveTextContent("3");
  });

  it("renders an action slot", () => {
    renderAt("/saved", <button>פעולה</button>);
    expect(screen.getByRole("button", { name: "פעולה" })).toBeInTheDocument();
  });
});
