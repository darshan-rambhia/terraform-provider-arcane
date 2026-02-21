package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the Arcane API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Config holds the client configuration.
type Config struct {
	URL    string
	APIKey string
}

// New creates a new Arcane API client.
func New(cfg Config) (*Client, error) {
	baseURL := strings.TrimSuffix(cfg.URL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("arcane URL is required")
	}

	return &Client{
		BaseURL: baseURL,
		APIKey:  cfg.APIKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Request represents an API request.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Body   interface{}
	Result interface{}
}

// Do executes an API request.
func (c *Client) Do(ctx context.Context, req *Request) error {
	// Build URL
	fullURL := c.BaseURL + req.Path
	if len(req.Query) > 0 {
		fullURL += "?" + req.Query.Encode()
	}

	// Build request body
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("X-API-Key", c.APIKey)
	}

	// Execute request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		apiErr.StatusCode = resp.StatusCode
		return &apiErr
	}

	// Parse response
	if req.Result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, req.Result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// APIError represents an API error response.
type APIError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"message"`
	Detail     string `json:"detail"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("API error (status %d): %s - %s", e.StatusCode, e.Message, e.Detail)
	}
	if e.Message != "" {
		return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error (status %d)", e.StatusCode)
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}

// esc escapes a string for safe inclusion in URL path segments.
func esc(s string) string {
	return url.PathEscape(s)
}

// PaginatedResponse wraps paginated API responses.
type PaginatedResponse[T any] struct {
	Success    bool       `json:"success"`
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// Pagination contains pagination metadata.
type Pagination struct {
	TotalPages   int `json:"totalPages"`
	TotalItems   int `json:"totalItems"`
	CurrentPage  int `json:"currentPage"`
	ItemsPerPage int `json:"itemsPerPage"`
}

// SingleResponse wraps single-item API responses.
type SingleResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

// EnvironmentClient provides environment-scoped operations.
type EnvironmentClient struct {
	client        *Client
	environmentID string
}

// ForEnvironment returns a client scoped to a specific environment.
func (c *Client) ForEnvironment(envID string) *EnvironmentClient {
	return &EnvironmentClient{
		client:        c,
		environmentID: envID,
	}
}

// Environment represents an Arcane environment.
type Environment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	APIURL      string `json:"apiUrl,omitempty"`
	Description string `json:"description,omitempty"`
	UseAPIKey   bool   `json:"use_api_key"`
	AccessToken string `json:"access_token,omitempty"`
	APIKey      string `json:"apiKey,omitempty"` // Returned when regenerating API key
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// EnvironmentCreateRequest represents a request to create an environment.
type EnvironmentCreateRequest struct {
	Name        string `json:"name"`
	APIURL      string `json:"apiUrl"`
	Description string `json:"description,omitempty"`
	UseAPIKey   bool   `json:"use_api_key,omitempty"`
}

// EnvironmentUpdateRequest represents a request to update an environment.
type EnvironmentUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	UseAPIKey   *bool  `json:"use_api_key,omitempty"`
}

// ListEnvironments returns all environments.
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	var result PaginatedResponse[Environment]
	err := c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments",
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetEnvironment returns an environment by ID.
func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var result SingleResponse[Environment]
	err := c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(id),
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetEnvironmentByName returns an environment by name.
func (c *Client) GetEnvironmentByName(ctx context.Context, name string) (*Environment, error) {
	envs, err := c.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if env.Name == name {
			return &env, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "environment not found"}
}

// CreateEnvironment creates a new environment.
func (c *Client) CreateEnvironment(ctx context.Context, req *EnvironmentCreateRequest) (*Environment, error) {
	var result SingleResponse[Environment]
	err := c.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments",
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateEnvironment updates an environment.
func (c *Client) UpdateEnvironment(ctx context.Context, id string, req *EnvironmentUpdateRequest) (*Environment, error) {
	var result SingleResponse[Environment]
	err := c.Do(ctx, &Request{
		Method: http.MethodPut,
		Path:   "/api/environments/" + esc(id),
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteEnvironment deletes an environment.
func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	return c.Do(ctx, &Request{
		Method: http.MethodDelete,
		Path:   "/api/environments/" + esc(id),
	})
}

// RegenerateEnvironmentAPIKey regenerates the API key for an environment.
// This returns a new API key with the arc_ prefix that agents use for authentication.
func (c *Client) RegenerateEnvironmentAPIKey(ctx context.Context, id string) (*Environment, error) {
	var result SingleResponse[Environment]
	err := c.Do(ctx, &Request{
		Method: http.MethodPut,
		Path:   "/api/environments/" + esc(id),
		Body:   map[string]bool{"regenerateApiKey": true},
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// Project represents an Arcane project (docker compose stack).
type Project struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	Path          string            `json:"path,omitempty"`
	Services      []ProjectService  `json:"services,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	EnvironmentID string            `json:"environment_id,omitempty"`
}

// ProjectService represents a service within a project.
type ProjectService struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Image  string `json:"image,omitempty"`
}

// ListProjects returns all projects in an environment.
func (ec *EnvironmentClient) ListProjects(ctx context.Context) ([]Project, error) {
	var result PaginatedResponse[Project]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/projects",
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetProject returns a project by ID.
func (ec *EnvironmentClient) GetProject(ctx context.Context, projectID string) (*Project, error) {
	var result SingleResponse[Project]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/projects/" + esc(projectID),
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetProjectByName returns a project by name.
func (ec *EnvironmentClient) GetProjectByName(ctx context.Context, name string) (*Project, error) {
	projects, err := ec.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "project not found"}
}

// ProjectDeployRequest represents a request to deploy a project.
type ProjectDeployRequest struct {
	// Pull images before deploying
	Pull bool `json:"pull,omitempty"`
	// Force recreate containers
	ForceRecreate bool `json:"force_recreate,omitempty"`
	// Remove orphan containers
	RemoveOrphans bool `json:"remove_orphans,omitempty"`
}

// DeployProject deploys (starts) a project.
func (ec *EnvironmentClient) DeployProject(ctx context.Context, projectID string, req *ProjectDeployRequest) error {
	if req == nil {
		req = &ProjectDeployRequest{}
	}
	return ec.client.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/projects/" + esc(projectID) + "/up",
		Body:   req,
	})
}

// RedeployProject redeploys a project.
func (ec *EnvironmentClient) RedeployProject(ctx context.Context, projectID string, req *ProjectDeployRequest) error {
	if req == nil {
		req = &ProjectDeployRequest{}
	}
	return ec.client.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/projects/" + esc(projectID) + "/redeploy",
		Body:   req,
	})
}

// StopProject stops a project.
func (ec *EnvironmentClient) StopProject(ctx context.Context, projectID string) error {
	return ec.client.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/projects/" + esc(projectID) + "/down",
	})
}

// ContainerDetail represents detailed container runtime information.
type ContainerDetail struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Image  string          `json:"image,omitempty"`
	Status string          `json:"status"`
	Health string          `json:"health,omitempty"`
	Ports  []ContainerPort `json:"ports,omitempty"`
}

// ContainerPort represents a container port mapping.
type ContainerPort struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// GetProjectContainers returns detailed container information for a project.
func (ec *EnvironmentClient) GetProjectContainers(ctx context.Context, projectID string) ([]ContainerDetail, error) {
	var result PaginatedResponse[ContainerDetail]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/projects/" + esc(projectID) + "/containers",
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// TestEnvironment tests connectivity to an environment's agent.
func (c *Client) TestEnvironment(ctx context.Context, id string) error {
	return c.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments/" + esc(id) + "/test",
	})
}

// GetContainer returns a single container by ID within an environment.
func (ec *EnvironmentClient) GetContainer(ctx context.Context, containerID string) (*ContainerDetail, error) {
	var result SingleResponse[ContainerDetail]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/containers/" + esc(containerID),
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetContainerByName returns a container by name within an environment.
// Searches across all projects in the environment.
func (ec *EnvironmentClient) GetContainerByName(ctx context.Context, name string) (*ContainerDetail, error) {
	projects, err := ec.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		containers, err := ec.GetProjectContainers(ctx, p.ID)
		if err != nil {
			continue
		}
		for _, c := range containers {
			if c.Name == name {
				return &c, nil
			}
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "container not found"}
}

// ContainerRegistry represents a container registry configuration.
type ContainerRegistry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	AuthType string `json:"auth_type,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ContainerRegistryCreateRequest represents a request to create a container registry.
type ContainerRegistryCreateRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	AuthType string `json:"auth_type,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ContainerRegistryUpdateRequest represents a request to update a container registry.
type ContainerRegistryUpdateRequest struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	AuthType string `json:"auth_type,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ListContainerRegistries returns all container registries.
func (c *Client) ListContainerRegistries(ctx context.Context) ([]ContainerRegistry, error) {
	var result PaginatedResponse[ContainerRegistry]
	err := c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/container-registries",
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetContainerRegistry returns a container registry by ID.
func (c *Client) GetContainerRegistry(ctx context.Context, id string) (*ContainerRegistry, error) {
	var result SingleResponse[ContainerRegistry]
	err := c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/container-registries/" + esc(id),
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateContainerRegistry creates a new container registry.
func (c *Client) CreateContainerRegistry(ctx context.Context, req *ContainerRegistryCreateRequest) (*ContainerRegistry, error) {
	var result SingleResponse[ContainerRegistry]
	err := c.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/container-registries",
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateContainerRegistry updates a container registry.
func (c *Client) UpdateContainerRegistry(ctx context.Context, id string, req *ContainerRegistryUpdateRequest) (*ContainerRegistry, error) {
	var result SingleResponse[ContainerRegistry]
	err := c.Do(ctx, &Request{
		Method: http.MethodPut,
		Path:   "/api/container-registries/" + esc(id),
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteContainerRegistry deletes a container registry.
func (c *Client) DeleteContainerRegistry(ctx context.Context, id string) error {
	return c.Do(ctx, &Request{
		Method: http.MethodDelete,
		Path:   "/api/container-registries/" + esc(id),
	})
}

// GitRepository represents a git repository configuration.
type GitRepository struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Branch      string `json:"branch,omitempty"`
	AuthType    string `json:"auth_type,omitempty"`
	Credentials string `json:"credentials,omitempty"`
}

// GitRepositoryCreateRequest represents a request to create a git repository.
type GitRepositoryCreateRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Branch      string `json:"branch,omitempty"`
	AuthType    string `json:"auth_type,omitempty"`
	Credentials string `json:"credentials,omitempty"`
}

// GitRepositoryUpdateRequest represents a request to update a git repository.
type GitRepositoryUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	URL         string `json:"url,omitempty"`
	Branch      string `json:"branch,omitempty"`
	AuthType    string `json:"auth_type,omitempty"`
	Credentials string `json:"credentials,omitempty"`
}

// ListGitRepositories returns all git repositories.
func (c *Client) ListGitRepositories(ctx context.Context) ([]GitRepository, error) {
	var result PaginatedResponse[GitRepository]
	err := c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/gitops/repositories",
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetGitRepository returns a git repository by ID.
func (c *Client) GetGitRepository(ctx context.Context, id string) (*GitRepository, error) {
	var result SingleResponse[GitRepository]
	err := c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/gitops/repositories/" + esc(id),
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateGitRepository creates a new git repository.
func (c *Client) CreateGitRepository(ctx context.Context, req *GitRepositoryCreateRequest) (*GitRepository, error) {
	var result SingleResponse[GitRepository]
	err := c.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/gitops/repositories",
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateGitRepository updates a git repository.
func (c *Client) UpdateGitRepository(ctx context.Context, id string, req *GitRepositoryUpdateRequest) (*GitRepository, error) {
	var result SingleResponse[GitRepository]
	err := c.Do(ctx, &Request{
		Method: http.MethodPut,
		Path:   "/api/gitops/repositories/" + esc(id),
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteGitRepository deletes a git repository.
func (c *Client) DeleteGitRepository(ctx context.Context, id string) error {
	return c.Do(ctx, &Request{
		Method: http.MethodDelete,
		Path:   "/api/gitops/repositories/" + esc(id),
	})
}

// GitOpsSync represents a GitOps sync configuration for an environment.
type GitOpsSync struct {
	ID             string `json:"id"`
	EnvironmentID  string `json:"environment_id,omitempty"`
	RepositoryID   string `json:"repository_id"`
	Path           string `json:"path,omitempty"`
	Branch         string `json:"branch,omitempty"`
	ComposeFile    string `json:"compose_file,omitempty"`
	SyncInterval   string `json:"sync_interval,omitempty"`
	AutoSync       bool   `json:"auto_sync"`
	LastSyncAt     string `json:"last_sync_at,omitempty"`
	LastSyncCommit string `json:"last_sync_commit,omitempty"`
}

// GitOpsSyncCreateRequest represents a request to create a GitOps sync.
type GitOpsSyncCreateRequest struct {
	RepositoryID string `json:"repository_id"`
	Path         string `json:"path,omitempty"`
	Branch       string `json:"branch,omitempty"`
	ComposeFile  string `json:"compose_file,omitempty"`
	SyncInterval string `json:"sync_interval,omitempty"`
	AutoSync     bool   `json:"auto_sync,omitempty"`
}

// GitOpsSyncUpdateRequest represents a request to update a GitOps sync.
type GitOpsSyncUpdateRequest struct {
	RepositoryID string `json:"repository_id,omitempty"`
	Path         string `json:"path,omitempty"`
	Branch       string `json:"branch,omitempty"`
	ComposeFile  string `json:"compose_file,omitempty"`
	SyncInterval string `json:"sync_interval,omitempty"`
	AutoSync     *bool  `json:"auto_sync,omitempty"`
}

// ListGitOpsSyncs returns all GitOps syncs for an environment.
func (ec *EnvironmentClient) ListGitOpsSyncs(ctx context.Context) ([]GitOpsSync, error) {
	var result PaginatedResponse[GitOpsSync]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/gitops-syncs",
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetGitOpsSync returns a GitOps sync by ID.
func (ec *EnvironmentClient) GetGitOpsSync(ctx context.Context, syncID string) (*GitOpsSync, error) {
	var result SingleResponse[GitOpsSync]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/gitops-syncs/" + esc(syncID),
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateGitOpsSync creates a new GitOps sync.
func (ec *EnvironmentClient) CreateGitOpsSync(ctx context.Context, req *GitOpsSyncCreateRequest) (*GitOpsSync, error) {
	var result SingleResponse[GitOpsSync]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/gitops-syncs",
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateGitOpsSync updates a GitOps sync.
func (ec *EnvironmentClient) UpdateGitOpsSync(ctx context.Context, syncID string, req *GitOpsSyncUpdateRequest) (*GitOpsSync, error) {
	var result SingleResponse[GitOpsSync]
	err := ec.client.Do(ctx, &Request{
		Method: http.MethodPut,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/gitops-syncs/" + esc(syncID),
		Body:   req,
		Result: &result,
	})
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteGitOpsSync deletes a GitOps sync.
func (ec *EnvironmentClient) DeleteGitOpsSync(ctx context.Context, syncID string) error {
	return ec.client.Do(ctx, &Request{
		Method: http.MethodDelete,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/gitops-syncs/" + esc(syncID),
	})
}

// TriggerGitOpsSync manually triggers a sync operation.
func (ec *EnvironmentClient) TriggerGitOpsSync(ctx context.Context, syncID string) error {
	return ec.client.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   "/api/environments/" + esc(ec.environmentID) + "/gitops-syncs/" + esc(syncID) + "/trigger",
	})
}
