import { getAuthToken } from "@/lib/auth-token";

/** Base path for versioned REST API (same origin). */
export const BASE_URL = "/api/v1";

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const headers = new Headers(options?.headers);
  if (!headers.has("Content-Type") && options?.body) {
    headers.set("Content-Type", "application/json");
  }
  if (!headers.has("Authorization")) {
    const token = await getAuthToken().catch(() => null);
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
  }
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });
  if (res.status === 401) {
    const freshToken = await getAuthToken(true).catch(() => null);
    if (freshToken) {
      headers.set("Authorization", `Bearer ${freshToken}`);
      const retry = await fetch(`${BASE_URL}${path}`, { ...options, headers });
      if (!retry.ok) {
        const body = await retry.json().catch(() => ({ error: "Unknown error" }));
        throw new ApiError(retry.status, body.error);
      }
      if (retry.status === 204) return undefined as T;
      return retry.json();
    }
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "Unknown error" }));
    throw new ApiError(res.status, body.error);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface Manufacturer {
  id: number;
  name: string;
  name_he?: string;
}

export interface Model {
  id: number;
  name: string;
  name_he?: string;
}

export interface Search {
  id: number;
  name: string;
  source: string;
  manufacturer_id: number;
  manufacturer_name: string;
  model_id: number;
  model_name: string;
  year_min: number;
  year_max: number;
  price_min: number;
  price_max: number;
  engine_min_cc: number;
  max_km: number;
  max_hand: number;
  keywords: string;
  exclude_keys: string;
  active: boolean;
  created_at: string;
  /** any | private | commercial */
  seller_filter?: string;
  gear_box?: string;
  price_only?: boolean;
  photo_only?: boolean;
  /** Total listings found for this search; from API when supported. */
  listings_count?: number;
  /** True when this is the user's first search, prompting push opt-in. */
  push_prompt?: boolean;
  stats?: {
    total: number;
    new_24h: number;
    avg_price: number;
  };
}

export interface CreateSearchRequest {
  name?: string;
  source: string;
  manufacturer: number;
  model: number;
  year_min: number;
  year_max: number;
  price_min?: number;
  price_max: number;
  engine_min_cc?: number;
  max_km?: number;
  max_hand?: number;
  keywords?: string;
  exclude_keys?: string;
  /** any (default), private, commercial */
  seller_filter?: string;
  gear_box?: string;
  price_only?: boolean;
  photo_only?: boolean;
}

export interface Listing {
  token: string;
  manufacturer: string;
  model: string;
  sub_model?: string;
  year: number;
  price: number;
  km: number;
  hand: number;
  city: string;
  page_link: string;
  image_url?: string;
  engine_volume?: number;
  horse_power?: number;
  engine_type?: string;
  gear_box?: string;
  description?: string;
  fitness_score?: number;
  median_price?: number;
  base_price?: number;
  cohort_size?: number;
  deal_score?: number;
  first_seen_at: string;
  posted_at?: string;
  /** Present when API includes bookmark state */
  saved?: boolean;
  /** User dismissed this listing from the new / notifications feed */
  seen?: boolean;
  /** From Yad2 bucket: true = dealer/commercial, false = private, absent when unknown. */
  is_commercial?: boolean | null;
  /** Set when the listing disappeared from the source but is bookmarked (likely sold). */
  removed_at?: string;
  /** Reasons the listing was flagged as suspicious. */
  suspicious_reasons?: string[];
}

export interface SearchStatsResponse {
  total: number;
  new_24h: number;
  avg_price: number;
  min_price: number;
  max_price: number;
}

export interface ListingsResponse {
  items: Listing[];
  total: number;
  limit: number;
  offset: number;
}

export interface RefreshResponse {
  items: Listing[];
  total: number;
  removed: number;
}

export interface ListingsParams {
  limit?: number;
  offset?: number;
  sort?: string;
}

export const api = {
  catalog: {
    manufacturers: (q?: string) =>
      fetchAPI<Manufacturer[]>(
        `/catalog/manufacturers${q ? `?q=${encodeURIComponent(q)}` : ""}`,
      ),
    models: (mfrId: number, q?: string) =>
      fetchAPI<Model[]>(
        `/catalog/manufacturers/${mfrId}/models${q ? `?q=${encodeURIComponent(q)}` : ""}`,
      ),
  },
  searches: {
    list: () => fetchAPI<Search[]>("/searches"),
    get: (id: number) => fetchAPI<Search>(`/searches/${id}`),
    create: (data: CreateSearchRequest) =>
      fetchAPI<Search>("/searches", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (id: number, data: Partial<CreateSearchRequest>) =>
      fetchAPI<Search>(`/searches/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    delete: (id: number) =>
      fetchAPI<void>(`/searches/${id}`, { method: "DELETE" }),
    pause: (id: number) =>
      fetchAPI<void>(`/searches/${id}/pause`, { method: "POST" }),
    resume: (id: number) =>
      fetchAPI<void>(`/searches/${id}/resume`, { method: "POST" }),
    stats: (id: number) =>
      fetchAPI<SearchStatsResponse>(`/searches/${id}/stats`),
  },
  listing: (token: string) => fetchAPI<Listing>(`/listings/${encodeURIComponent(token)}`),
  priceHistory: (token: string) =>
    fetchAPI<{ items: { price: number; observed_at: string }[] }>(`/listings/${encodeURIComponent(token)}/price-history`),
  markListingSeen: (token: string) =>
    fetchAPI<void>(`/listings/${encodeURIComponent(token)}/seen`, {
      method: "POST",
    }),
  unmarkListingSeen: (token: string) =>
    fetchAPI<void>(`/listings/${encodeURIComponent(token)}/seen`, {
      method: "DELETE",
    }),
  saved: {
    list: (params?: ListingsParams) => {
      const query = new URLSearchParams();
      if (params?.limit !== undefined) query.set("limit", String(params.limit));
      if (params?.offset !== undefined) query.set("offset", String(params.offset));
      const qs = query.toString();
      return fetchAPI<ListingsResponse>(`/saved${qs ? `?${qs}` : ""}`);
    },
    save: (token: string) =>
      fetchAPI<void>(`/listings/${encodeURIComponent(token)}/save`, {
        method: "POST",
      }),
    remove: (token: string) =>
      fetchAPI<void>(`/listings/${encodeURIComponent(token)}/save`, {
        method: "DELETE",
      }),
  },
  history: (params?: ListingsParams) => {
    const query = new URLSearchParams();
    if (params?.limit !== undefined) query.set("limit", String(params.limit));
    if (params?.offset !== undefined) query.set("offset", String(params.offset));
    if (params?.sort) query.set("sort", params.sort);
    const qs = query.toString();
    return fetchAPI<ListingsResponse>(`/history${qs ? `?${qs}` : ""}`);
  },
  listings: (searchId: number, params?: ListingsParams) => {
    const query = new URLSearchParams();
    if (params?.limit !== undefined) query.set("limit", String(params.limit));
    if (params?.offset !== undefined) query.set("offset", String(params.offset));
    if (params?.sort) query.set("sort", params.sort);
    const qs = query.toString();
    return fetchAPI<ListingsResponse>(
      `/searches/${searchId}/listings${qs ? `?${qs}` : ""}`,
    );
  },
  refreshListings: (searchId: number) =>
    fetchAPI<RefreshResponse>(`/searches/${searchId}/refresh`, {
      method: "POST",
    }),
  searchActivity: (searchId: number, days = 14) =>
    fetchAPI<{ days: DailyListingCount[] }>(
      `/searches/${searchId}/activity?days=${days}`,
    ),
};

export interface DailyListingCount {
  day: string; // YYYY-MM-DD
  count: number;
}

export interface NotificationCount {
  count: number;
}

export const notificationsApi = {
  count: () => fetchAPI<NotificationCount>("/notifications/count"),
  list: (params?: ListingsParams) => {
    const query = new URLSearchParams();
    if (params?.limit !== undefined) query.set("limit", String(params.limit));
    if (params?.offset !== undefined)
      query.set("offset", String(params.offset));
    const qs = query.toString();
    return fetchAPI<ListingsResponse>(
      `/notifications${qs ? `?${qs}` : ""}`,
    );
  },
  markSeen: () =>
    fetchAPI<void>("/notifications/seen", { method: "POST" }),
};

export interface AdminStats {
  db: {
    file_size_bytes: number;
    file_size_human: string;
  };
  tables: Record<string, number>;
  runtime: {
    goroutines: number;
    mem_alloc_mb: number;
    mem_sys_mb: number;
    uptime: string;
  };
  http: {
    requests_total: number;
    status_2xx: number;
    status_4xx: number;
    status_5xx: number;
    avg_duration_ms: number;
  };
  pool?: {
    max_open_connections: number;
    open_connections: number;
    in_use: number;
    idle: number;
    wait_count: number;
    wait_duration: string;
  };
  vitals?: VitalsSummary[];
}

export interface VitalsSummary {
  name: string;
  p50: number;
  p75: number;
  p95: number;
  count: number;
  good: number;
  needs_improvement: number;
  poor: number;
}

export interface AdminDelivery {
  status: string; // sent | failed | dropped | dead_lettered
  alert_type: string; // instant | price_drop | digest | daily
  sent_at: string;
}

export interface AdminListing extends Listing {
  chat_id: number;
  search_id: number;
  search_name?: string;
  /** Latest Telegram-delivery outcome; absent = no delivery recorded (matched-only). */
  delivered?: AdminDelivery;
}

export interface AdminListingsResponse {
  items: AdminListing[];
  total: number;
  limit: number;
  offset: number;
}

export interface AdminDayActivity {
  date: string;
  new_listings: number;
  price_drops: number;
  new_users: number;
}

export interface AdminActivityResponse {
  items: AdminDayActivity[];
}

export interface SyncUserStatusResult {
  activated: number;
  deactivated: number;
}

export interface PurgeResult {
  table: string;
  deleted: number;
}

export interface VacuumResult {
  status: string;
  size_after?: string;
  size_bytes?: number;
}

export interface AdminSearch {
  id: number;
  chat_id: number;
  username?: string;
  name: string;
  source: string;
  manufacturer: number;
  model: number;
  year_min: number;
  year_max: number;
  price_min: number;
  price_max: number;
  engine_min_cc: number;
  max_km: number;
  max_hand: number;
  keywords?: string;
  exclude_keys?: string;
  /** any | private | commercial */
  seller_filter?: string;
  gear_box?: string;
  price_only?: boolean;
  photo_only?: boolean;
  active: boolean;
  created_at: string;
}

export interface AdminSearchesResponse {
  items: AdminSearch[];
  total: number;
}

export interface AdminLinkedTelegram {
  chat_id: number;
  username: string;
  channel_id: string;
}

export interface AdminUser {
  chat_id: number;
  username: string;
  channel: string;
  channel_id: string;
  active: boolean;
  tier: string;
  language: string;
  created_at: string;
  linked_telegram?: AdminLinkedTelegram;
}

export interface AdminUsersResponse {
  items: AdminUser[];
  total: number;
}

export interface LogEntry {
  time: string;
  level: string;
  message: string;
  component: string;
  attrs: Record<string, string>;
}

export interface LogsResponse {
  items: LogEntry[];
}

export interface AdminCycleEntry {
  id: number;
  started_at: string;
  duration_ms: number;
  searches: number;
  listings_fetched: number;
  listings_matched: number;
  notifications: number;
  error_message?: string;
  status: string;
}

export interface AdminCyclesResponse {
  items: AdminCycleEntry[];
}

export const adminApi = {
  stats: () => fetchAPI<AdminStats>("/admin/stats"),
  listings: (params?: ListingsParams & { search_id?: number; chat_id?: number }) => {
    const query = new URLSearchParams();
    if (params?.limit !== undefined) query.set("limit", String(params.limit));
    if (params?.offset !== undefined) query.set("offset", String(params.offset));
    if (params?.search_id) query.set("search_id", String(params.search_id));
    if (params?.chat_id) query.set("chat_id", String(params.chat_id));
    const qs = query.toString();
    return fetchAPI<AdminListingsResponse>(`/admin/listings${qs ? `?${qs}` : ""}`);
  },
  deleteListing: (token: string, chatId: number) =>
    fetchAPI<void>(`/admin/listings/${encodeURIComponent(token)}`, {
      method: "DELETE",
      body: JSON.stringify({ chat_id: chatId }),
    }),
  searches: () => fetchAPI<AdminSearchesResponse>("/admin/searches"),
  deleteSearch: (id: number) =>
    fetchAPI<void>(`/admin/searches/${id}`, { method: "DELETE" }),
  users: () => fetchAPI<AdminUsersResponse>("/admin/users"),
  deleteUser: (chatId: number) =>
    fetchAPI<void>(`/admin/users/${chatId}`, { method: "DELETE" }),
  setUserActive: (chatId: number, active: boolean) =>
    fetchAPI<{ chat_id: number; active: boolean }>(`/admin/users/${chatId}`, {
      method: "PATCH",
      body: JSON.stringify({ active }),
    }),
  purgeTable: (table: string) =>
    fetchAPI<PurgeResult>("/admin/purge", {
      method: "POST",
      body: JSON.stringify({ table }),
    }),
  vacuum: () =>
    fetchAPI<VacuumResult>("/admin/vacuum", { method: "POST" }),
  resetAll: () =>
    fetchAPI<{ tables: Record<string, number>; total: number }>("/admin/reset-all", { method: "POST" }),
  activity: (days?: number) =>
    fetchAPI<AdminActivityResponse>(`/admin/activity${days ? `?days=${days}` : ""}`),
  cycles: (limit?: number) =>
    fetchAPI<AdminCyclesResponse>(`/admin/cycles${limit ? `?limit=${limit}` : ""}`),
  logs: (limit?: number) =>
    fetchAPI<LogsResponse>(`/admin/logs${limit ? `?limit=${limit}` : ""}`),
  setLogLevel: (level: string) =>
    fetchAPI<{ level: string }>("/admin/logs/level", {
      method: "PUT",
      body: JSON.stringify({ level }),
    }),
  getLogLevel: () => fetchAPI<{ level: string }>("/admin/logs/level"),
  syncUserStatus: () =>
    fetchAPI<SyncUserStatusResult>("/admin/sync-user-status", { method: "POST" }),
};

export interface UserProfile {
  email: string;
  is_admin: boolean;
}

export const userApi = {
  me: () => fetchAPI<UserProfile>("/me"),
};

export interface SchedulerStatus {
  last_cycle_at: string | null;
  last_cycle_duration_ms: number;
  last_cycle_status: string;
  next_cycle_at: string | null;
  polling_interval_seconds: number;
  searches: number;
  listings_fetched: number;
  listings_matched: number;
  notifications: number;
}

export const schedulerApi = {
  status: () => fetchAPI<SchedulerStatus>("/scheduler/status"),
};

export interface SearchCycleStatsItem {
  search_id: number;
  search_name: string;
  cycle_at: string;
  feed_size: number;
  matched: number;
  new_listings: number;
  km_filtered: number;
  delivered: number;
  price_drops: number;
  wrong_model: number;
  year_out: number;
  price_out: number;
  km_over: number;
  hand_over: number;
  engine_cc: number;
  seller: number;
  other_filter: number;
  dropped: number;
  score_min: number | null;
  score_max: number | null;
  score_avg: number | null;
  price_min: number | null;
  price_max: number | null;
}

export const cycleStatsApi = {
  list: () => fetchAPI<{ items: SearchCycleStatsItem[] }>("/searches/cycle-stats"),
};

export interface TelegramLinkResponse {
  link: string;
  expires_in_seconds: number;
}

export interface TelegramStatus {
  connected: boolean;
  telegram_username: string | null;
}

export const pushApi = {
  vapidKey: () => fetchAPI<{ public_key: string }>("/push/vapid-key"),
  subscribe: (subscription: PushSubscriptionJSON) =>
    fetchAPI<void>("/push/subscribe", {
      method: "POST",
      body: JSON.stringify(subscription),
    }),
  unsubscribe: (endpoint: string) =>
    fetchAPI<void>("/push/subscribe", {
      method: "DELETE",
      body: JSON.stringify({ endpoint }),
    }),
};

export const telegramApi = {
  status: () => fetchAPI<TelegramStatus>("/telegram/status"),
  createLink: () =>
    fetchAPI<TelegramLinkResponse>("/telegram/link", { method: "POST" }),
};

async function fetchGuestAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const headers = new Headers(options?.headers);
  if (!headers.has("Content-Type") && options?.body) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "Unknown error" }));
    throw new ApiError(res.status, body.error);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface InstantSearchRequest {
  source: string;
  manufacturer: number;
  model: number;
  year_min?: number;
  year_max?: number;
  price_min?: number;
  price_max?: number;
  max_km?: number;
  max_hand?: number;
  engine_min_cc?: number;
  gear_box?: string;
  price_only?: boolean;
  photo_only?: boolean;
}

export interface InstantSearchResponse {
  items: Listing[];
  total: number;
}

export const guestApi = {
  instantSearch: (data: InstantSearchRequest) =>
    fetchGuestAPI<InstantSearchResponse>("/guest/instant-search", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  catalog: {
    manufacturers: (q?: string) =>
      fetchGuestAPI<Manufacturer[]>(
        `/catalog/manufacturers${q ? `?q=${encodeURIComponent(q)}` : ""}`,
      ),
    models: (mfrId: number, q?: string) =>
      fetchGuestAPI<Model[]>(
        `/catalog/manufacturers/${mfrId}/models${q ? `?q=${encodeURIComponent(q)}` : ""}`,
      ),
  },
};

export { ApiError };
