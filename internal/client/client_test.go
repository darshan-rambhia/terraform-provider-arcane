package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Client creation & validation ─────────────────────────────────────────────

func TestNew_GivenValidConfig_ReturnsClient(t *testing.T) {
	t.Parallel()
	c, err := New(Config{URL: "http://localhost:8000", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.BaseURL != "http://localhost:8000" {
		t.Errorf("expected BaseURL http://localhost:8000, got %s", c.BaseURL)
	}
	if c.APIKey != "test-key" {
		t.Errorf("expected APIKey test-key, got %s", c.APIKey)
	}
}

func TestNew_GivenEmptyURL_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := New(Config{URL: ""})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestNew_GivenTrailingSlash_TrimsSlash(t *testing.T) {
	t.Parallel()
	c, err := New(Config{URL: "http://localhost:8000/"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.BaseURL != "http://localhost:8000" {
		t.Errorf("expected trailing slash trimmed, got %s", c.BaseURL)
	}
}

// ─── Request building ─────────────────────────────────────────────────────────

func TestDo_GivenBody_MarshalsJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["name"] != "test" {
			t.Errorf("expected name=test in body, got %s", req["name"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.Do(context.Background(), &Request{
		Method: http.MethodPost,
		Path:   "/test",
		Body:   map[string]string{"name": "test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_GivenQueryParams_AppendsToURL(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected query param page=2, got %s", r.URL.Query().Get("page"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	q := make(map[string][]string)
	q["page"] = []string{"2"}
	err := c.Do(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/test",
		Query:  q,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_GivenAPIKey_SetsHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "my-key" {
			t.Errorf("expected X-API-Key header, got %s", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, APIKey: "my-key", HTTPClient: srv.Client()}
	err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_GivenNoAPIKey_OmitsHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" {
			t.Error("expected no X-API-Key header when key is empty")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Response parsing ─────────────────────────────────────────────────────────

func TestDo_GivenSingleResponse_ParsesData(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(SingleResponse[Environment]{
			Success: true,
			Data:    Environment{ID: "env-1", Name: "test"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	var result SingleResponse[Environment]
	err := c.Do(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/test",
		Result: &result,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data.ID != "env-1" {
		t.Errorf("expected ID env-1, got %s", result.Data.ID)
	}
}

func TestDo_GivenPaginatedResponse_ParsesDataAndPagination(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[Environment]{
			Success: true,
			Data:    []Environment{{ID: "env-1"}, {ID: "env-2"}},
			Pagination: Pagination{
				TotalPages:  1,
				TotalItems:  2,
				CurrentPage: 1,
			},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	var result PaginatedResponse[Environment]
	err := c.Do(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/test",
		Result: &result,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Data))
	}
	if result.Pagination.TotalItems != 2 {
		t.Errorf("expected TotalItems 2, got %d", result.Pagination.TotalItems)
	}
}

func TestDo_GivenEmptyBody_SkipsParsing(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	var result SingleResponse[Environment]
	err := c.Do(context.Background(), &Request{
		Method: http.MethodDelete,
		Path:   "/test",
		Result: &result,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_GivenMalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	var result SingleResponse[Environment]
	err := c.Do(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/test",
		Result: &result,
	})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// ─── Error handling ───────────────────────────────────────────────────────────

func TestDo_Given404_ReturnsAPIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Message: "not found"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/test"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var apiErr *APIError
	if !isAPIError(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestDo_Given500_ReturnsAPIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIError{Message: "internal error", Detail: "something broke"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/test"})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	var apiErr *APIError
	if !isAPIError(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
}

func TestDo_GivenNonJSONError_ReturnsFallbackError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/test"})
	if err == nil {
		t.Fatal("expected error for non-JSON error response")
	}
	// Should be a plain error, not an APIError
	if IsNotFound(err) {
		t.Error("should not be a not-found error")
	}
}

func TestIsNotFound_Given404APIError_ReturnsTrue(t *testing.T) {
	t.Parallel()
	err := &APIError{StatusCode: 404, Message: "not found"}
	if !IsNotFound(err) {
		t.Error("expected IsNotFound to return true for 404")
	}
}

func TestIsNotFound_Given500APIError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	err := &APIError{StatusCode: 500, Message: "internal error"}
	if IsNotFound(err) {
		t.Error("expected IsNotFound to return false for 500")
	}
}

func TestIsNotFound_GivenNonAPIError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("some error")
	if IsNotFound(err) {
		t.Error("expected IsNotFound to return false for non-API error")
	}
}

func TestAPIError_Error_GivenMessageAndDetail(t *testing.T) {
	t.Parallel()
	err := &APIError{StatusCode: 422, Message: "validation error", Detail: "name required"}
	expected := "API error (status 422): validation error - name required"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestAPIError_Error_GivenMessageOnly(t *testing.T) {
	t.Parallel()
	err := &APIError{StatusCode: 404, Message: "not found"}
	expected := "API error (status 404): not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestAPIError_Error_GivenNoMessageOrDetail(t *testing.T) {
	t.Parallel()
	err := &APIError{StatusCode: 500}
	expected := "API error (status 500)"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

// ─── Environment CRUD methods ─────────────────────────────────────────────────

func TestListEnvironments_ReturnsAll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/environments" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[Environment]{
			Success: true,
			Data:    []Environment{{ID: "env-1", Name: "prod"}, {ID: "env-2", Name: "staging"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	envs, err := c.ListEnvironments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 2 {
		t.Errorf("expected 2 environments, got %d", len(envs))
	}
}

func TestGetEnvironment_ReturnsEnv(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[Environment]{
			Success: true,
			Data:    Environment{ID: "env-1", Name: "prod"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	env, err := c.GetEnvironment(context.Background(), "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "prod" {
		t.Errorf("expected name prod, got %s", env.Name)
	}
}

func TestGetEnvironmentByName_GivenExistingName_ReturnsEnv(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[Environment]{
			Success: true,
			Data:    []Environment{{ID: "env-1", Name: "prod"}, {ID: "env-2", Name: "staging"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	env, err := c.GetEnvironmentByName(context.Background(), "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.ID != "env-2" {
		t.Errorf("expected ID env-2, got %s", env.ID)
	}
}

func TestGetEnvironmentByName_GivenMissingName_Returns404(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[Environment]{
			Success: true,
			Data:    []Environment{{ID: "env-1", Name: "prod"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.GetEnvironmentByName(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !IsNotFound(err) {
		t.Error("expected IsNotFound to be true")
	}
}

func TestCreateEnvironment_SendsRequestAndReturnsEnv(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req EnvironmentCreateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "new-env" {
			t.Errorf("expected name new-env, got %s", req.Name)
		}
		json.NewEncoder(w).Encode(SingleResponse[Environment]{
			Success: true,
			Data:    Environment{ID: "env-new", Name: req.Name},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	env, err := c.CreateEnvironment(context.Background(), &EnvironmentCreateRequest{
		Name:   "new-env",
		APIURL: "http://test:3553",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.ID != "env-new" {
		t.Errorf("expected ID env-new, got %s", env.ID)
	}
}

func TestUpdateEnvironment_SendsRequestAndReturnsUpdated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/environments/env-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[Environment]{
			Success: true,
			Data:    Environment{ID: "env-1", Name: "updated"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	env, err := c.UpdateEnvironment(context.Background(), "env-1", &EnvironmentUpdateRequest{Name: "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "updated" {
		t.Errorf("expected name updated, got %s", env.Name)
	}
}

func TestDeleteEnvironment_SendsDelete(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/environments/env-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.DeleteEnvironment(context.Background(), "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegenerateEnvironmentAPIKey_ReturnsNewKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["regenerateApiKey"] != true {
			t.Error("expected regenerateApiKey=true in body")
		}
		json.NewEncoder(w).Encode(SingleResponse[Environment]{
			Success: true,
			Data:    Environment{ID: "env-1", APIKey: "arc_new_key"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	env, err := c.RegenerateEnvironmentAPIKey(context.Background(), "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.APIKey != "arc_new_key" {
		t.Errorf("expected APIKey arc_new_key, got %s", env.APIKey)
	}
}

func TestTestEnvironment_GivenConnected_ReturnsNil(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/environments/env-1/test" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.TestEnvironment(context.Background(), "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── EnvironmentClient project methods ────────────────────────────────────────

func TestListProjects_ReturnsAll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[Project]{
			Success: true,
			Data:    []Project{{ID: "proj-1", Name: "webapp"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	projects, err := ec.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

func TestGetProject_ReturnsProject(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/projects/proj-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[Project]{
			Success: true,
			Data:    Project{ID: "proj-1", Name: "webapp", Status: "running"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	p, err := ec.GetProject(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != "running" {
		t.Errorf("expected status running, got %s", p.Status)
	}
}

func TestGetProjectByName_GivenExistingName_ReturnsProject(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[Project]{
			Success: true,
			Data:    []Project{{ID: "proj-1", Name: "webapp"}, {ID: "proj-2", Name: "api"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	p, err := ec.GetProjectByName(context.Background(), "api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != "proj-2" {
		t.Errorf("expected ID proj-2, got %s", p.ID)
	}
}

func TestGetProjectByName_GivenMissingName_Returns404(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[Project]{
			Success: true,
			Data:    []Project{{ID: "proj-1", Name: "webapp"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	_, err := ec.GetProjectByName(context.Background(), "nonexistent")
	if !IsNotFound(err) {
		t.Error("expected not-found error")
	}
}

func TestDeployProject_SendsPost(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/environments/env-1/projects/proj-1/up" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req ProjectDeployRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Pull {
			t.Error("expected pull=true")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	err := ec.DeployProject(context.Background(), "proj-1", &ProjectDeployRequest{Pull: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeployProject_GivenNilRequest_UsesDefaults(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	err := ec.DeployProject(context.Background(), "proj-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedeployProject_SendsPost(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/projects/proj-1/redeploy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	err := ec.RedeployProject(context.Background(), "proj-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopProject_SendsPost(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/environments/env-1/projects/proj-1/down" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	err := ec.StopProject(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetProjectContainers_ReturnsContainers(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/projects/proj-1/containers" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[ContainerDetail]{
			Success: true,
			Data: []ContainerDetail{
				{
					ID:     "c1",
					Name:   "webapp-1",
					Image:  "nginx:latest",
					Status: "running",
					Health: "healthy",
					Ports: []ContainerPort{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	containers, err := ec.GetProjectContainers(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].Health != "healthy" {
		t.Errorf("expected health healthy, got %s", containers[0].Health)
	}
	if len(containers[0].Ports) != 1 || containers[0].Ports[0].HostPort != 8080 {
		t.Error("expected port mapping 8080:80")
	}
}

// ─── Container registry methods ───────────────────────────────────────────────

func TestListContainerRegistries_ReturnsAll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/container-registries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[ContainerRegistry]{
			Success: true,
			Data:    []ContainerRegistry{{ID: "reg-1", Name: "ghcr", URL: "https://ghcr.io"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	regs, err := c.ListContainerRegistries(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(regs) != 1 {
		t.Errorf("expected 1 registry, got %d", len(regs))
	}
}

func TestGetContainerRegistry_ReturnsRegistry(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/container-registries/reg-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[ContainerRegistry]{
			Success: true,
			Data:    ContainerRegistry{ID: "reg-1", Name: "ghcr", URL: "https://ghcr.io"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	reg, err := c.GetContainerRegistry(context.Background(), "reg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Name != "ghcr" {
		t.Errorf("expected name ghcr, got %s", reg.Name)
	}
}

func TestCreateContainerRegistry_ReturnsCreated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req ContainerRegistryCreateRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(SingleResponse[ContainerRegistry]{
			Success: true,
			Data:    ContainerRegistry{ID: "reg-new", Name: req.Name, URL: req.URL},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	reg, err := c.CreateContainerRegistry(context.Background(), &ContainerRegistryCreateRequest{
		Name: "dockerhub",
		URL:  "https://index.docker.io",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.ID != "reg-new" {
		t.Errorf("expected ID reg-new, got %s", reg.ID)
	}
}

func TestUpdateContainerRegistry_ReturnsUpdated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/container-registries/reg-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[ContainerRegistry]{
			Success: true,
			Data:    ContainerRegistry{ID: "reg-1", Name: "updated", URL: "https://updated.io"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	reg, err := c.UpdateContainerRegistry(context.Background(), "reg-1", &ContainerRegistryUpdateRequest{Name: "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Name != "updated" {
		t.Errorf("expected name updated, got %s", reg.Name)
	}
}

func TestDeleteContainerRegistry_SendsDelete(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/container-registries/reg-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.DeleteContainerRegistry(context.Background(), "reg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Git repository methods ──────────────────────────────────────────────────

func TestListGitRepositories_ReturnsAll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/gitops/repositories" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[GitRepository]{
			Success: true,
			Data:    []GitRepository{{ID: "repo-1", Name: "infra", URL: "https://github.com/test/infra"}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	repos, err := c.ListGitRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(repos))
	}
}

func TestGetGitRepository_ReturnsRepo(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/gitops/repositories/repo-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[GitRepository]{
			Success: true,
			Data:    GitRepository{ID: "repo-1", Name: "infra", Branch: "main"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	repo, err := c.GetGitRepository(context.Background(), "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Branch != "main" {
		t.Errorf("expected branch main, got %s", repo.Branch)
	}
}

func TestCreateGitRepository_ReturnsCreated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req GitRepositoryCreateRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(SingleResponse[GitRepository]{
			Success: true,
			Data:    GitRepository{ID: "repo-new", Name: req.Name, URL: req.URL, Branch: "main"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	repo, err := c.CreateGitRepository(context.Background(), &GitRepositoryCreateRequest{
		Name: "new-repo",
		URL:  "https://github.com/test/new",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.ID != "repo-new" {
		t.Errorf("expected ID repo-new, got %s", repo.ID)
	}
}

func TestUpdateGitRepository_ReturnsUpdated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/gitops/repositories/repo-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[GitRepository]{
			Success: true,
			Data:    GitRepository{ID: "repo-1", Name: "updated", Branch: "develop"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	repo, err := c.UpdateGitRepository(context.Background(), "repo-1", &GitRepositoryUpdateRequest{Name: "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Name != "updated" {
		t.Errorf("expected name updated, got %s", repo.Name)
	}
}

func TestDeleteGitRepository_SendsDelete(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/gitops/repositories/repo-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	err := c.DeleteGitRepository(context.Background(), "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── GitOps sync methods ─────────────────────────────────────────────────────

func TestListGitOpsSyncs_ReturnsAll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/gitops-syncs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[GitOpsSync]{
			Success: true,
			Data:    []GitOpsSync{{ID: "sync-1", RepositoryID: "repo-1", AutoSync: true}},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	syncs, err := ec.ListGitOpsSyncs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syncs) != 1 {
		t.Errorf("expected 1 sync, got %d", len(syncs))
	}
}

func TestGetGitOpsSync_ReturnsSync(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/gitops-syncs/sync-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[GitOpsSync]{
			Success: true,
			Data:    GitOpsSync{ID: "sync-1", RepositoryID: "repo-1", Branch: "main", ComposeFile: "docker-compose.yml"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	sync, err := ec.GetGitOpsSync(context.Background(), "sync-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sync.ComposeFile != "docker-compose.yml" {
		t.Errorf("expected compose file docker-compose.yml, got %s", sync.ComposeFile)
	}
}

func TestCreateGitOpsSync_ReturnsCreated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/environments/env-1/gitops-syncs" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var req GitOpsSyncCreateRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(SingleResponse[GitOpsSync]{
			Success: true,
			Data: GitOpsSync{
				ID:           "sync-new",
				RepositoryID: req.RepositoryID,
				Branch:       "main",
				ComposeFile:  "docker-compose.yml",
				AutoSync:     req.AutoSync,
			},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	sync, err := ec.CreateGitOpsSync(context.Background(), &GitOpsSyncCreateRequest{
		RepositoryID: "repo-1",
		AutoSync:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sync.ID != "sync-new" {
		t.Errorf("expected ID sync-new, got %s", sync.ID)
	}
	if !sync.AutoSync {
		t.Error("expected auto_sync to be true")
	}
}

func TestUpdateGitOpsSync_ReturnsUpdated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/environments/env-1/gitops-syncs/sync-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[GitOpsSync]{
			Success: true,
			Data:    GitOpsSync{ID: "sync-1", RepositoryID: "repo-1", AutoSync: false},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	autoSync := false
	sync, err := ec.UpdateGitOpsSync(context.Background(), "sync-1", &GitOpsSyncUpdateRequest{AutoSync: &autoSync})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sync.AutoSync {
		t.Error("expected auto_sync to be false")
	}
}

func TestDeleteGitOpsSync_SendsDelete(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/environments/env-1/gitops-syncs/sync-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	err := ec.DeleteGitOpsSync(context.Background(), "sync-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTriggerGitOpsSync_SendsPost(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/environments/env-1/gitops-syncs/sync-1/trigger" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	err := ec.TriggerGitOpsSync(context.Background(), "sync-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Container lookup methods ─────────────────────────────────────────────────

func TestGetContainer_ReturnsContainer(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/env-1/containers/c1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SingleResponse[ContainerDetail]{
			Success: true,
			Data:    ContainerDetail{ID: "c1", Name: "webapp", Status: "running", Health: "healthy"},
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	container, err := ec.GetContainer(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.Name != "webapp" {
		t.Errorf("expected name webapp, got %s", container.Name)
	}
}

func TestGetContainerByName_ReturnsContainer(t *testing.T) {
	t.Parallel()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/environments/env-1/projects":
			json.NewEncoder(w).Encode(PaginatedResponse[Project]{
				Success: true,
				Data:    []Project{{ID: "proj-1", Name: "webapp"}},
			})
		case "/api/environments/env-1/projects/proj-1/containers":
			callCount++
			json.NewEncoder(w).Encode(PaginatedResponse[ContainerDetail]{
				Success: true,
				Data: []ContainerDetail{
					{ID: "c1", Name: "nginx", Status: "running"},
					{ID: "c2", Name: "postgres", Status: "running"},
				},
			})
		}
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ec := c.ForEnvironment("env-1")
	container, err := ec.GetContainerByName(context.Background(), "postgres")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.ID != "c2" {
		t.Errorf("expected ID c2, got %s", container.ID)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func isAPIError(err error, target **APIError) bool {
	return errors.As(err, target)
}
