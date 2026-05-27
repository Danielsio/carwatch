import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LogsTab } from "./LogsTab";

let mockedLogs = [
  {
    time: "2026-05-28T00:00:00Z",
    level: "ERROR",
    message: "deliver alert failed",
    component: "broker-consumer",
    attrs: { chat_id: "2000000000004" },
  },
  {
    time: "2026-05-28T00:00:01Z",
    level: "INFO",
    message: "notifier worker started",
    component: "notifier",
    attrs: {},
  },
];

vi.mock("@/hooks/useLogStream", () => ({
  useLogStream: () => ({
    logs: mockedLogs,
    connected: true,
    clear: vi.fn(),
  }),
}));

vi.mock("@/lib/api", () => ({
  adminApi: {
    getLogLevel: vi.fn(async () => ({ level: "INFO" })),
    setLogLevel: vi.fn(async (level: string) => ({ level })),
  },
}));

describe("LogsTab", () => {
  beforeEach(() => {
    mockedLogs = [
      {
        time: "2026-05-28T00:00:00Z",
        level: "ERROR",
        message: "deliver alert failed",
        component: "broker-consumer",
        attrs: { chat_id: "2000000000004" },
      },
      {
        time: "2026-05-28T00:00:01Z",
        level: "INFO",
        message: "notifier worker started",
        component: "notifier",
        attrs: {},
      },
    ];
    Element.prototype.scrollIntoView = vi.fn();
  });

  it("renders dynamic components and can exclude one from view", async () => {
    const view = render(<LogsTab active />);

    expect(screen.getAllByText("broker-consumer").length).toBeGreaterThan(0);
    expect(screen.getAllByText("notifier").length).toBeGreaterThan(0);
    expect(screen.getByText("deliver alert failed")).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: "broker-consumer" })[0]);

    expect(screen.queryByText("deliver alert failed")).not.toBeInTheDocument();
    expect(screen.getByText("notifier worker started")).toBeInTheDocument();

    mockedLogs = [mockedLogs[1]];
    view.rerender(<LogsTab active />);
    expect(screen.queryByRole("button", { name: "broker-consumer" })).not.toBeInTheDocument();

    mockedLogs = [
      {
        time: "2026-05-28T00:00:02Z",
        level: "ERROR",
        message: "deliver alert failed again",
        component: "broker-consumer",
        attrs: {},
      },
      mockedLogs[0],
    ];
    view.rerender(<LogsTab active />);
    fireEvent.click(screen.getAllByRole("button", { name: "broker-consumer" })[0]);
    expect(screen.getByText("deliver alert failed again")).toBeInTheDocument();
  });
});
