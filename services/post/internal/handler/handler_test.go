package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"post/internal/handler"
	"post/internal/repository"
	"post/internal/service"
)

func newTestMux() *http.ServeMux {
	repo := repository.NewInMemory()
	svc := service.New(repo)
	h := handler.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	mux := http.NewServeMux()
	h.SetRoutes(mux)
	return mux
}

func do(mux *http.ServeMux, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (body %q)", err, rr.Body.String())
	}
	return body
}

func withUser(userID string) map[string]string {
	return map[string]string{"X-User-ID": userID}
}

func createPost(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	rr := do(mux, http.MethodPost, "/posts", `{"title":"T","content":"C"}`, withUser("123"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rr.Code, rr.Body.String())
	}
	body := decodeJSON(t, rr)
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("created post missing id")
	}
	return id
}

func TestCreatePost(t *testing.T) {
	mux := newTestMux()
	id := createPost(t, mux)
	if id == "" {
		t.Fatal("no id")
	}

	t.Run("without header", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts", `{"title":"T","content":"C"}`, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("empty title", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts", `{"title":"","content":"C"}`, withUser("123"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts", `{not json`, withUser("123"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

func TestGetPost(t *testing.T) {
	mux := newTestMux()

	t.Run("existing seed", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/posts/p1", "", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		body := decodeJSON(t, rr)
		if body["author_id"] != "123" {
			t.Errorf("author_id = %v, want 123", body["author_id"])
		}
	})

	t.Run("missing", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/posts/xyz", "", nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

func TestListPosts(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodGet, "/posts", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := decodeJSON(t, rr)
	posts, ok := body["posts"].([]any)
	if !ok || len(posts) != 2 {
		t.Fatalf("posts field = %v, want list of 2", body["posts"])
	}
}

func TestUpdatePost(t *testing.T) {
	mux := newTestMux()

	t.Run("author updates", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/posts/p1", `{"title":"Renamed"}`, withUser("123"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if body["title"] != "Renamed" {
			t.Errorf("title = %v, want Renamed", body["title"])
		}
	})

	t.Run("non-author forbidden", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/posts/p1", `{"title":"Hijack"}`, withUser("999"))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("no header", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/posts/p1", `{"title":"X"}`, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("unknown post", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/posts/xyz", `{"title":"X"}`, withUser("123"))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

func TestDeletePost(t *testing.T) {
	mux := newTestMux()

	t.Run("non-author forbidden", func(t *testing.T) {
		rr := do(mux, http.MethodDelete, "/posts/p1", "", withUser("999"))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("author deletes", func(t *testing.T) {
		rr := do(mux, http.MethodDelete, "/posts/p1", "", withUser("123"))
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})

	t.Run("already gone", func(t *testing.T) {
		rr := do(mux, http.MethodDelete, "/posts/p1", "", withUser("123"))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

func TestLikePost(t *testing.T) {
	mux := newTestMux()
	id := createPost(t, mux)

	t.Run("first like", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts/"+id+"/like", "", withUser("u1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if body["likes"] != float64(1) {
			t.Errorf("likes = %v, want 1", body["likes"])
		}
	})

	t.Run("repeat like idempotent", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts/"+id+"/like", "", withUser("u1"))
		body := decodeJSON(t, rr)
		if body["likes"] != float64(1) {
			t.Errorf("likes = %v, want 1 (idempotent)", body["likes"])
		}
	})

	t.Run("second user", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts/"+id+"/like", "", withUser("u2"))
		body := decodeJSON(t, rr)
		if body["likes"] != float64(2) {
			t.Errorf("likes = %v, want 2", body["likes"])
		}
	})

	t.Run("without header", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts/"+id+"/like", "", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("unknown post", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/posts/xyz/like", "", withUser("u1"))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

func TestHealth(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodGet, "/health", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := decodeJSON(t, rr)
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
}
