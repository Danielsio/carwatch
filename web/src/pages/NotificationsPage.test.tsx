import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NotificationsPage } from "./NotificationsPage";

const mockListing = {
  token: "tok1",
  manufacturer: "Toyota",
  model: "Corolla",
  year: 2021,
  price: 120000,
  km: 50000,
  hand: 2,
  city: "Tel Aviv",
  page_link: "https://yad2.co.il/item/tok1",
  image_url: "https://img.yad2.co.il/tok1.jpg",
  first_seen_at: "2024-01-01T00:00:00Z",
  saved: false,
  seen: false,
};

const mockMarkSeenMutate = vi.fn();
const mockMarkOneMutate = vi.fn();

let notificationsReturn: {
  data: { items: typeof mockListing[]; total: number } | undefined;
  isLoading: boolean;
  isError: boolean;
} = {
  data: { items: [mockListing], total: 1 },
  isLoading: false,
  isError: false,
};

vi.mock("@/hooks/useNotifications", () => ({
  useNotifications: () => notificationsReturn,
  useMarkNotificationsSeen: () => ({
    mutate: mockMarkSeenMutate,
    isPending: false,
  }),
  useNotificationCount: () => ({ data: { count: 1 } }),
}));

vi.mock("@/hooks/useListingSeen", () => ({
  useMarkListingSeen: () => ({
    mutate: mockMarkOneMutate,
    isPending: false,
  }),
}));

vi.mock("@/hooks/usePageTitle", () => ({
  usePageTitle: vi.fn(),
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <NotificationsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("NotificationsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    notificationsReturn = {
      data: { items: [mockListing], total: 1 },
      isLoading: false,
      isError: false,
    };
  });

  it("renders the inbox tab header", () => {
    renderPage();
    expect(screen.getByRole("link", { name: /התראות/ })).toBeInTheDocument();
  });

  it("renders empty state when no notifications", () => {
    notificationsReturn = {
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    };
    renderPage();
    expect(screen.getByText("אין התראות חדשות")).toBeInTheDocument();
    expect(screen.getByText("חזרה ללוח הבקרה")).toBeInTheDocument();
  });

  it("renders notifications with total count", () => {
    renderPage();
    expect(screen.getByText("(1 חדשות)")).toBeInTheDocument();
  });

  it("shows mark-all-seen button when notifications exist", () => {
    renderPage();
    expect(screen.getByText("סמן הכל כנקרא")).toBeInTheDocument();
  });

  it("calls markSeen when mark-all button is clicked", async () => {
    const user = userEvent.setup();
    renderPage();
    const btn = screen.getByText("סמן הכל כנקרא");
    await user.click(btn);
    expect(mockMarkSeenMutate).toHaveBeenCalledTimes(1);
  });

  it("shows loading skeletons while loading", () => {
    notificationsReturn = { data: undefined, isLoading: true, isError: false };
    const { container } = renderPage();
    const skeletons = container.querySelectorAll(
      '[data-slot="skeleton"], [class*="shimmer"], [class*="skeleton"]',
    );
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows error state on fetch failure", () => {
    notificationsReturn = {
      data: undefined,
      isLoading: false,
      isError: true,
    };
    renderPage();
    expect(screen.getByText("שגיאה בטעינת ההתראות")).toBeInTheDocument();
  });

  it("renders dismiss button on each notification card", () => {
    renderPage();
    const dismissBtn = screen.getByLabelText("סמן כנצפה והסר מהחדשות");
    expect(dismissBtn).toBeInTheDocument();
  });

  it("calls markListingSeen when dismiss button is clicked", async () => {
    const user = userEvent.setup();
    renderPage();
    const dismissBtn = screen.getByLabelText("סמן כנצפה והסר מהחדשות");
    await user.click(dismissBtn);
    expect(mockMarkOneMutate).toHaveBeenCalledWith("tok1");
  });
});
