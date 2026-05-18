package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMarkBotPollingAlive(t *testing.T) {
	s := New()
	s.RecordSuccess()
	s.MarkBotPollingAlive()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["bot_polling"] != true {
		t.Errorf("bot_polling = %v, want true", resp["bot_polling"])
	}
}

func TestMarkBotPollingDead(t *testing.T) {
	s := New()
	s.RecordSuccess()
	s.MarkBotPollingAlive()
	s.MarkBotPollingDead()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["bot_polling"] != false {
		t.Errorf("bot_polling = %v, want false", resp["bot_polling"])
	}
}
