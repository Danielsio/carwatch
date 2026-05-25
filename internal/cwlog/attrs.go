package cwlog

import (
	"log/slog"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// ListingAttrs returns structured slog attributes for a raw listing.
// Zero-value optional fields are omitted to keep logs compact.
func ListingAttrs(l model.RawListing) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("token", l.Token),
		slog.String("manufacturer", l.Manufacturer),
		slog.String("model", l.Model),
		slog.Int("year", l.Year),
		slog.Int("price", l.Price),
	}
	if l.Km > 0 {
		attrs = append(attrs, slog.Int("km", l.Km))
	}
	if l.SubModel != "" {
		attrs = append(attrs, slog.String("sub_model", l.SubModel))
	}
	if l.SubModelID > 0 {
		attrs = append(attrs, slog.Int("sub_model_id", l.SubModelID))
	}
	return attrs
}

// SearchAttrs returns structured slog attributes for a search.
func SearchAttrs(s storage.Search) []slog.Attr {
	return []slog.Attr{
		slog.Int64("search_id", s.ID),
		slog.Int64("chat_id", s.ChatID),
		slog.String("search_name", s.Name),
		slog.String("source", s.Source),
	}
}

// ErrorEvent returns the standardized error/impact/action_taken attribute
// triple for error logging. Every error log should include these fields
// to make debugging actionable.
func ErrorEvent(err error, impact, actionTaken string) []slog.Attr {
	errMsg := "<nil>"
	if err != nil {
		errMsg = err.Error()
	}
	return []slog.Attr{
		slog.String("error", errMsg),
		slog.String("impact", impact),
		slog.String("action_taken", actionTaken),
	}
}

// PriceDropAttrs returns structured slog attributes for a price change event.
func PriceDropAttrs(token string, oldPrice, newPrice int) []slog.Attr {
	return []slog.Attr{
		slog.String("token", token),
		slog.Int("old_price", oldPrice),
		slog.Int("new_price", newPrice),
		slog.Int("price_change", newPrice-oldPrice),
	}
}
