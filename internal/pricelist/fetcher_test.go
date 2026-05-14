package pricelist

import (
	"context"
	"fmt"
	"testing"
)

type mockHTTPDoer struct {
	body       []byte
	statusCode int
	err        error
}

func (m *mockHTTPDoer) Get(_ context.Context, _ string) (*HTTPResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &HTTPResult{Body: m.body, StatusCode: m.statusCode}, nil
}

func TestFetch_Success_HTMLLabel(t *testing.T) {
	html := `<div>מחיר בסיס ₪ 120,000</div>`
	client := &mockHTTPDoer{body: []byte(html), statusCode: 200}

	result := fetch(context.Background(), client, 12345, 2020)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.BasePrice != 120000 {
		t.Errorf("expected base_price=120000, got %d", result.BasePrice)
	}
}

func TestFetch_Success_JSONBasePrice(t *testing.T) {
	html := `<script>{"basePrice": 95000, "title": "Honda Civic"}</script>`
	client := &mockHTTPDoer{body: []byte(html), statusCode: 200}

	result := fetch(context.Background(), client, 12345, 2020)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.BasePrice != 95000 {
		t.Errorf("expected base_price=95000, got %d", result.BasePrice)
	}
}

func TestFetch_HTTP400(t *testing.T) {
	client := &mockHTTPDoer{body: nil, statusCode: 400}

	result := fetch(context.Background(), client, 12345, 2020)
	if result.Error == "" {
		t.Fatal("expected error for HTTP 400")
	}
	if result.BasePrice != 0 {
		t.Errorf("expected base_price=0, got %d", result.BasePrice)
	}
}

func TestFetch_HTTPError(t *testing.T) {
	client := &mockHTTPDoer{err: fmt.Errorf("connection refused")}

	result := fetch(context.Background(), client, 12345, 2020)
	if result.Error == "" {
		t.Fatal("expected error for connection failure")
	}
}

func TestFetch_NoPriceInHTML(t *testing.T) {
	html := `<html><body>No price data here</body></html>`
	client := &mockHTTPDoer{body: []byte(html), statusCode: 200}

	result := fetch(context.Background(), client, 12345, 2020)
	if result.Error == "" {
		t.Fatal("expected error when no price found")
	}
	if result.BasePrice != 0 {
		t.Errorf("expected base_price=0, got %d", result.BasePrice)
	}
}

func TestExtractPriceFromHTML_LabelWithCommas(t *testing.T) {
	html := `מחיר בסיס ₪ 1,234,567`
	result := extractPriceFromHTML(html)
	if result.BasePrice != 1234567 {
		t.Errorf("expected 1234567, got %d", result.BasePrice)
	}
}

func TestExtractPriceFromHTML_JSONFallback(t *testing.T) {
	html := `no label here but {"basePrice": 55000}`
	result := extractPriceFromHTML(html)
	if result.BasePrice != 55000 {
		t.Errorf("expected 55000, got %d", result.BasePrice)
	}
}

func TestExtractPriceFromHTML_PriceTooLow(t *testing.T) {
	html := `מחיר בסיס ₪ 500`
	result := extractPriceFromHTML(html)
	if result.Error == "" {
		t.Error("expected error for price below 1000 threshold")
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"120,000", 120000},
		{"55000", 55000},
		{" 1,234,567 ", 1234567},
		{"", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := parsePrice(tt.input)
		if got != tt.expected {
			t.Errorf("parsePrice(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
