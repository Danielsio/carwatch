import { http, HttpResponse } from "msw";
import { BASE_URL } from "@/lib/api";

export const handlers = [
  http.get(`${BASE_URL}/me`, () =>
    HttpResponse.json({ email: "test@example.com", is_admin: false }),
  ),

  http.get(`${BASE_URL}/searches`, () => HttpResponse.json([])),

  http.get(`${BASE_URL}/notifications/count`, () =>
    HttpResponse.json({ count: 0 }),
  ),

  http.get(`${BASE_URL}/catalog/manufacturers`, () =>
    HttpResponse.json([
      { id: 19, name: "Toyota", name_he: "טויוטה" },
      { id: 27, name: "Mazda", name_he: "מאזדה" },
    ]),
  ),

  http.get(`${BASE_URL}/catalog/manufacturers/:id/models`, () =>
    HttpResponse.json([
      { id: 10226, name: "Corolla", name_he: "קורולה" },
    ]),
  ),

  http.get(`${BASE_URL}/searches/cycle-stats`, () =>
    HttpResponse.json({ items: [] }),
  ),

  http.get(`${BASE_URL}/scheduler/status`, () =>
    HttpResponse.json({
      last_cycle_at: null,
      next_cycle_at: null,
      polling_interval_seconds: 900,
      searches: 0,
    }),
  ),

  http.get(`${BASE_URL}/saved`, () =>
    HttpResponse.json({ items: [], total: 0, limit: 20, offset: 0 }),
  ),

  http.get(`${BASE_URL}/history`, () =>
    HttpResponse.json({ items: [], total: 0, limit: 20, offset: 0 }),
  ),

  http.get(`${BASE_URL}/notifications`, () =>
    HttpResponse.json({ items: [], total: 0, limit: 20, offset: 0 }),
  ),

  http.get(`${BASE_URL}/push/vapid-key`, () =>
    HttpResponse.json({ key: "test-vapid-key" }),
  ),

  http.get(`${BASE_URL}/telegram/status`, () =>
    HttpResponse.json({ connected: false }),
  ),
];
