package winwin

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
	"golang.org/x/net/html"
)

// cardClassPatterns are substrings that indicate a listing card container element.
var cardClassPatterns = []string{"card", "item", "listing", "product", "result", "vehicle"}

var (
	priceRe      = regexp.MustCompile("(?:" + "\u20aa" + `|ש"ח|NIS|ILS|ILS\s*)\s*([\d,]+)|([\d,]+)\s*` + "\u20aa")
	kmRe         = regexp.MustCompile(`([\d,]+)\s*(?:ק"|ק״מ|קמ|km|Km|KM)`)
	handRe       = regexp.MustCompile(`(?:יד|בעלות|בעלים)\s*(\d+)`)
	yearSuffixRe = regexp.MustCompile(`\b(19[89]\d|20\d{2})\s*$`)
	trailingIDRe = regexp.MustCompile(`/(\d{5,})(?:[/?#]|$)`)
	midPathIDRe  = regexp.MustCompile(`/vehicles/[^/]+/(\d{4,})(?:[/?#]|$)`)
)

func ParseListingsPage(r io.Reader) ([]model.RawListing, error) {
	if r == nil {
		return nil, nil
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	htmlStr := string(raw)
	if looksLikeChallenge(htmlStr) {
		return nil, fetcher.ErrChallenge
	}
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	seen := make(map[string]struct{})
	var out []model.RawListing
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if listing, ok := rawListingFromAnchor(n); ok {
				if _, dup := seen[listing.Token]; !dup {
					seen[listing.Token] = struct{}{}
					out = append(out, listing)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return out, nil
}

func looksLikeChallenge(htmlStr string) bool {
	h := strings.ToLower(htmlStr)
	return strings.Contains(h, "errors.edgesuite.net") ||
		strings.Contains(h, "an error occurred while processing your request") ||
		strings.Contains(h, "cf-browser-verification") || strings.Contains(h, "captcha") ||
		(strings.Contains(h, "access denied") && strings.Contains(h, "akamai"))
}

func rawListingFromAnchor(a *html.Node) (model.RawListing, bool) {
	href := attr(a, "href")
	if href == "" || href == "#" {
		return model.RawListing{}, false
	}
	token, ok := listingTokenFromHref(href)
	if !ok {
		return model.RawListing{}, false
	}
	lh := strings.ToLower(href)
	if !strings.Contains(lh, "vehicles") && !strings.Contains(lh, "vehicle") {
		return model.RawListing{}, false
	}
	pageLink := resolveWinWinURL(href)
	scope := listingCardScope(a)
	blob := strings.TrimSpace(textContent(scope))
	var listing model.RawListing
	listing.Token = token
	listing.PageLink = pageLink
	title := strings.TrimSpace(textContent(a))
	if title != "" {
		if sub := yearSuffixRe.FindStringSubmatch(title); len(sub) > 1 {
			if y, err := strconv.Atoi(sub[1]); err == nil {
				listing.Year = y
				title = strings.TrimSpace(strings.TrimSuffix(title, sub[1]))
			}
		}
		parts := strings.Fields(title)
		if len(parts) >= 2 {
			listing.Manufacturer = parts[0]
			listing.Model = strings.Join(parts[1:], " ")
		} else if len(parts) == 1 {
			listing.Manufacturer = parts[0]
		}
	}
	if p, ok := firstPriceInText(blob); ok {
		listing.Price = p
	}
	if km, ok := firstMatchInt(kmRe, blob); ok {
		listing.Km = km
	}
	if hand, ok := firstMatchInt(handRe, blob); ok {
		listing.Hand = hand
	}
	if city := extractCityHeuristic(blob); city != "" {
		listing.City = city
	}
	return listing, true
}

func resolveWinWinURL(href string) string {
	base, _ := url.Parse("https://www.winwin.co.il")
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

func listingCardScope(a *html.Node) *html.Node {
	// Walk up to 10 parents looking for an element whose class suggests a card container.
	n := a
	for i := 0; i < 10 && n.Parent != nil; i++ {
		n = n.Parent
		if looksLikeCardContainer(n) {
			slog.Debug("listingCardScope: matched card container by class", "level", i+1, "class", attr(n, "class"))
			return n
		}
	}

	// Fallback: walk exactly 6 parents from the anchor (original heuristic).
	slog.Debug("listingCardScope: no card class found, falling back to 6-parent heuristic")
	n = a
	for i := 0; i < 6 && n.Parent != nil; i++ {
		n = n.Parent
	}
	return n
}

// looksLikeCardContainer returns true if the node's class attribute contains
// a substring that commonly indicates a listing card container.
func looksLikeCardContainer(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	cls := strings.ToLower(attr(n, "class"))
	if cls == "" {
		return false
	}
	for _, pattern := range cardClassPatterns {
		if strings.Contains(cls, pattern) {
			return true
		}
	}
	return false
}

func listingTokenFromHref(href string) (string, bool) {
	if m := trailingIDRe.FindStringSubmatch(href); len(m) > 1 {
		return m[1], true
	}
	if m := midPathIDRe.FindStringSubmatch(href); len(m) > 1 {
		return m[1], true
	}
	return "", false
}

func attr(n *html.Node, key string) string {
	for _, at := range n.Attr {
		if strings.EqualFold(at.Key, key) {
			return at.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func firstPriceInText(s string) (int, bool) {
	for _, m := range priceRe.FindAllStringSubmatch(s, -1) {
		for _, grp := range m[1:] {
			if grp == "" {
				continue
			}
			digits := strings.ReplaceAll(grp, ",", "")
			n, err := strconv.Atoi(digits)
			if err == nil && n > 0 {
				return n, true
			}
		}
	}
	return 0, false
}

func firstMatchInt(re *regexp.Regexp, s string) (int, bool) {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0, false
	}
	digits := strings.ReplaceAll(m[1], ",", "")
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func extractCityHeuristic(blob string) string {
	for _, sep := range []string{"•", "|", "·", " - ", ","} {
		i := strings.Index(blob, sep)
		if i >= 0 {
			frag := strings.TrimSpace(blob[i+len(sep):])
			fields := strings.Fields(frag)
			var parts []string
			for _, w := range fields {
				if strings.Contains(w, "\u20aa") {
					break
				}
				parts = append(parts, w)
				if len(parts) >= 3 {
					break
				}
			}
			if s := strings.Join(parts, " "); s != "" {
				return s
			}
		}
	}
	return ""
}
