import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { PriceHistoryChart } from "./PriceHistoryChart";

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("PriceHistoryChart", () => {
  it("renders empty state when no records", async () => {
    server.use(
      http.get("/api/v1/listings/:token/price-history", () =>
        HttpResponse.json({ items: [] }),
      ),
    );

    render(<PriceHistoryChart token="tok-1" currentPrice={100000} />, {
      wrapper,
    });
    expect(
      await screen.findByText("אין היסטוריית מחירים"),
    ).toBeInTheDocument();
  });

  it("renders single-point message", async () => {
    server.use(
      http.get("/api/v1/listings/:token/price-history", () =>
        HttpResponse.json({
          items: [{ price: 120000, observed_at: "2026-06-01T00:00:00Z" }],
        }),
      ),
    );

    render(<PriceHistoryChart token="tok-2" currentPrice={120000} />, {
      wrapper,
    });
    expect(
      await screen.findByText("המחיר לא השתנה מאז שהמודעה הופיעה"),
    ).toBeInTheDocument();
  });

  it("renders SVG chart with multiple data points", async () => {
    server.use(
      http.get("/api/v1/listings/:token/price-history", () =>
        HttpResponse.json({
          items: [
            { price: 130000, observed_at: "2026-05-01T00:00:00Z" },
            { price: 125000, observed_at: "2026-05-15T00:00:00Z" },
            { price: 120000, observed_at: "2026-06-01T00:00:00Z" },
          ],
        }),
      ),
    );

    render(<PriceHistoryChart token="tok-3" currentPrice={120000} />, {
      wrapper,
    });

    const chart = await screen.findByRole("img", {
      name: "גרף היסטוריית מחירים",
    });
    expect(chart).toBeInTheDocument();
    expect(chart.tagName).toBe("svg");
    expect(chart.querySelectorAll("circle")).toHaveLength(3);
    expect(chart.querySelector("path")).toBeInTheDocument();
  });

  it("shows downward trend label when price drops", async () => {
    server.use(
      http.get("/api/v1/listings/:token/price-history", () =>
        HttpResponse.json({
          items: [
            { price: 150000, observed_at: "2026-05-01T00:00:00Z" },
            { price: 120000, observed_at: "2026-06-01T00:00:00Z" },
          ],
        }),
      ),
    );

    render(<PriceHistoryChart token="tok-4" currentPrice={120000} />, {
      wrapper,
    });
    expect(await screen.findByText("ירידה במחיר")).toBeInTheDocument();
  });
});
