const BASE_URL = "/api/v1";

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json", ...options?.headers },
    ...options,
  });
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
}

export interface Model {
  id: number;
  name: string;
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
  price_max: number;
  engine_min_cc: number;
  max_km: number;
  max_hand: number;
  keywords: string;
  exclude_keys: string;
  active: boolean;
  created_at: string;
}

export interface CreateSearchRequest {
  source: string;
  manufacturer: number;
  model: number;
  year_min: number;
  year_max: number;
  price_max: number;
  engine_min_cc?: number;
  max_km?: number;
  max_hand?: number;
  keywords?: string;
  exclude_keys?: string;
}

export interface Listing {
  token: string;
  manufacturer: string;
  model: string;
  year: number;
  price: number;
  km: number;
  hand: number;
  city: string;
  page_link: string;
  image_url?: string;
  fitness_score?: number;
  first_seen_at: string;
}

export interface ListingsResponse {
  items: Listing[];
  total: number;
  limit: number;
  offset: number;
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
  },
  listing: (token: string) => fetchAPI<Listing>(`/listings/${encodeURIComponent(token)}`),
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
}

export const adminApi = {
  stats: () => fetchAPI<AdminStats>("/admin/stats"),
};

export { ApiError };
