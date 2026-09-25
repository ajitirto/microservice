package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Health)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"ok"}`
	var gotMap, wantMap map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &gotMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &wantMap); err != nil {
		t.Fatal(err)
	}

	if gotMap["status"] != wantMap["status"] {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}
