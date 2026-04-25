package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/storage"
)

type ListingLister interface {
	ListListings(ctx context.Context, limit int) ([]storage.ListingRecord, error)
}

type Handler struct {
	store ListingLister
}

func NewHandler(store ListingLister) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'")
	w.Header().Set("Referrer-Policy", "no-referrer")

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	listings, err := h.store.ListListings(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to load listings", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, listings); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

var tmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"fmtPrice": format.Number,
	"fmtKm":    format.Number,
	"yad2Link": func(token string) string {
		return fmt.Sprintf("https://www.yad2.co.il/item/%s", token)
	},
}).Parse(`<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CarWatch Dashboard</title>
<style>
body { font-family: system-ui, sans-serif; margin: 20px; background: #f5f5f5; }
h1 { color: #333; }
table { border-collapse: collapse; width: 100%; background: white; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
th, td { padding: 8px 12px; text-align: left; border-bottom: 1px solid #eee; }
th { background: #2c3e50; color: white; position: sticky; top: 0; }
tr:hover { background: #f0f7ff; }
a { color: #2980b9; }
.price { font-weight: bold; color: #27ae60; }
.km { color: #7f8c8d; }
</style>
</head><body>
<h1>CarWatch - Listing History</h1>
<p>Showing {{len .}} listings</p>
<table>
<tr>
  <th>Car</th><th>Year</th><th>Price</th><th>Km</th><th>Hand</th><th>City</th><th>Search</th><th>Seen</th><th>Link</th>
</tr>
{{range .}}
<tr>
  <td>{{.Manufacturer}} {{.Model}}</td>
  <td>{{.Year}}</td>
  <td class="price">{{fmtPrice .Price}} &#8362;</td>
  <td class="km">{{fmtKm .Km}}</td>
  <td>{{.Hand}}</td>
  <td>{{.City}}</td>
  <td>{{.SearchName}}</td>
  <td>{{.FirstSeenAt.Format "2006-01-02 15:04"}}</td>
  <td><a href="{{.PageLink}}" target="_blank">View</a></td>
</tr>
{{end}}
</table>
</body></html>`))
