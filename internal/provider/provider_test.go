package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"arcane": providerserver.NewProtocol6WithError(New("test")()),
}

// writeJSON writes a JSON response with proper content type header.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeSingleResponse wraps data in SingleResponse format.
func writeSingleResponse[T any](w http.ResponseWriter, data T) {
	writeJSON(w, client.SingleResponse[T]{
		Success: true,
		Data:    data,
	})
}

// writePaginatedResponse wraps data in PaginatedResponse format.
func writePaginatedResponse[T any](w http.ResponseWriter, data []T) {
	writeJSON(w, client.PaginatedResponse[T]{
		Success: true,
		Data:    data,
		Pagination: client.Pagination{
			TotalPages:   1,
			TotalItems:   len(data),
			CurrentPage:  1,
			ItemsPerPage: len(data),
		},
	})
}

// MockServer creates a mock HTTP server that simulates Arcane API responses.
type MockServer struct {
	*httptest.Server
	Environments        map[string]*client.Environment
	Projects            map[string]map[string]*client.Project
	Containers          map[string]map[string][]client.ContainerDetail
	HealthyEnvs         map[string]bool // environments where agent is "connected"
	ContainerRegistries map[string]*client.ContainerRegistry
	GitRepositories     map[string]*client.GitRepository
	GitOpsSyncs         map[string]map[string]*client.GitOpsSync // envID -> syncID -> sync
}

// NewMockServer creates a new mock Arcane API server with properly wrapped responses.
func NewMockServer() *MockServer {
	ms := &MockServer{
		Environments:        make(map[string]*client.Environment),
		Projects:            make(map[string]map[string]*client.Project),
		Containers:          make(map[string]map[string][]client.ContainerDetail),
		HealthyEnvs:         make(map[string]bool),
		ContainerRegistries: make(map[string]*client.ContainerRegistry),
		GitRepositories:     make(map[string]*client.GitRepository),
		GitOpsSyncs:         make(map[string]map[string]*client.GitOpsSync),
	}

	mux := http.NewServeMux()

	// Environments list
	mux.HandleFunc("/api/environments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			envs := make([]client.Environment, 0, len(ms.Environments))
			for _, env := range ms.Environments {
				envs = append(envs, *env)
			}
			writePaginatedResponse(w, envs)
		case http.MethodPost:
			var req client.EnvironmentCreateRequest
			json.NewDecoder(r.Body).Decode(&req)
			env := &client.Environment{
				ID:          "env-" + req.Name,
				Name:        req.Name,
				APIURL:      req.APIURL,
				Description: req.Description,
				UseAPIKey:   req.UseAPIKey,
			}
			if req.UseAPIKey {
				env.AccessToken = "mock-token-" + req.Name
			}
			ms.Environments[env.ID] = env
			if ms.Projects[env.ID] == nil {
				ms.Projects[env.ID] = make(map[string]*client.Project)
			}
			ms.HealthyEnvs[env.ID] = true
			writeSingleResponse(w, *env)
		}
	})

	mux.HandleFunc("/api/environments/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/api/environments/"):]

		// Route to sub-handlers
		for envID := range ms.Environments {
			prefix := envID + "/projects"
			if strings.HasPrefix(path, prefix) {
				ms.handleProjectsEndpoint(w, r, envID, path[len(prefix):])
				return
			}
			if path == envID+"/test" {
				ms.handleTestEndpoint(w, r, envID)
				return
			}
			gsPrefix := envID + "/gitops-syncs"
			if strings.HasPrefix(path, gsPrefix) {
				ms.handleGitOpsSyncsEndpoint(w, r, envID, path[len(gsPrefix):])
				return
			}
			cPrefix := envID + "/containers/"
			if strings.HasPrefix(path, cPrefix) {
				containerID := path[len(cPrefix):]
				ms.handleContainerEndpoint(w, r, envID, containerID)
				return
			}
		}

		// Also check for projects on environments not yet created (pre-populated)
		for envID := range ms.Projects {
			prefix := envID + "/projects"
			if strings.HasPrefix(path, prefix) {
				ms.handleProjectsEndpoint(w, r, envID, path[len(prefix):])
				return
			}
		}

		// Check gitops-syncs for pre-populated environments
		for envID := range ms.GitOpsSyncs {
			gsPrefix := envID + "/gitops-syncs"
			if strings.HasPrefix(path, gsPrefix) {
				ms.handleGitOpsSyncsEndpoint(w, r, envID, path[len(gsPrefix):])
				return
			}
		}

		// Handle /api/environments/{id}
		envID := path
		env, exists := ms.Environments[envID]

		switch r.Method {
		case http.MethodGet:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, client.APIError{Message: "environment not found"})
				return
			}
			writeSingleResponse(w, *env)
		case http.MethodPut:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, client.APIError{Message: "environment not found"})
				return
			}

			// Check for regenerateApiKey request
			var rawReq map[string]interface{}
			json.NewDecoder(r.Body).Decode(&rawReq)
			if regen, ok := rawReq["regenerateApiKey"]; ok && regen == true {
				env.APIKey = "arc_regenerated_" + env.Name
				writeSingleResponse(w, *env)
				return
			}

			// Regular update
			if name, ok := rawReq["name"].(string); ok && name != "" {
				env.Name = name
			}
			if desc, ok := rawReq["description"].(string); ok {
				env.Description = desc
			}
			if useAPIKey, ok := rawReq["use_api_key"].(*bool); ok && useAPIKey != nil {
				env.UseAPIKey = *useAPIKey
			}
			writeSingleResponse(w, *env)
		case http.MethodDelete:
			delete(ms.Environments, envID)
			delete(ms.Projects, envID)
			delete(ms.HealthyEnvs, envID)
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Container registries list + create
	mux.HandleFunc("/api/container-registries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			regs := make([]client.ContainerRegistry, 0, len(ms.ContainerRegistries))
			for _, reg := range ms.ContainerRegistries {
				regs = append(regs, *reg)
			}
			writePaginatedResponse(w, regs)
		case http.MethodPost:
			var req client.ContainerRegistryCreateRequest
			json.NewDecoder(r.Body).Decode(&req)
			reg := &client.ContainerRegistry{
				ID:       "reg-" + req.Name,
				Name:     req.Name,
				URL:      req.URL,
				AuthType: req.AuthType,
				Username: req.Username,
			}
			ms.ContainerRegistries[reg.ID] = reg
			writeSingleResponse(w, *reg)
		}
	})

	// Container registries CRUD by ID
	mux.HandleFunc("/api/container-registries/", func(w http.ResponseWriter, r *http.Request) {
		regID := r.URL.Path[len("/api/container-registries/"):]
		reg, exists := ms.ContainerRegistries[regID]

		switch r.Method {
		case http.MethodGet:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, client.APIError{Message: "registry not found"})
				return
			}
			writeSingleResponse(w, *reg)
		case http.MethodPut:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, client.APIError{Message: "registry not found"})
				return
			}
			var req client.ContainerRegistryUpdateRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Name != "" {
				reg.Name = req.Name
			}
			if req.URL != "" {
				reg.URL = req.URL
			}
			if req.AuthType != "" {
				reg.AuthType = req.AuthType
			}
			if req.Username != "" {
				reg.Username = req.Username
			}
			writeSingleResponse(w, *reg)
		case http.MethodDelete:
			delete(ms.ContainerRegistries, regID)
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Git repositories list + create
	mux.HandleFunc("/api/gitops/repositories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			repos := make([]client.GitRepository, 0, len(ms.GitRepositories))
			for _, repo := range ms.GitRepositories {
				repos = append(repos, *repo)
			}
			writePaginatedResponse(w, repos)
		case http.MethodPost:
			var req client.GitRepositoryCreateRequest
			json.NewDecoder(r.Body).Decode(&req)
			repo := &client.GitRepository{
				ID:       "repo-" + req.Name,
				Name:     req.Name,
				URL:      req.URL,
				Branch:   req.Branch,
				AuthType: req.AuthType,
			}
			if repo.Branch == "" {
				repo.Branch = "main"
			}
			ms.GitRepositories[repo.ID] = repo
			writeSingleResponse(w, *repo)
		}
	})

	// Git repositories CRUD by ID
	mux.HandleFunc("/api/gitops/repositories/", func(w http.ResponseWriter, r *http.Request) {
		repoID := r.URL.Path[len("/api/gitops/repositories/"):]
		repo, exists := ms.GitRepositories[repoID]

		switch r.Method {
		case http.MethodGet:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, client.APIError{Message: "repository not found"})
				return
			}
			writeSingleResponse(w, *repo)
		case http.MethodPut:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, client.APIError{Message: "repository not found"})
				return
			}
			var req client.GitRepositoryUpdateRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Name != "" {
				repo.Name = req.Name
			}
			if req.URL != "" {
				repo.URL = req.URL
			}
			if req.Branch != "" {
				repo.Branch = req.Branch
			}
			if req.AuthType != "" {
				repo.AuthType = req.AuthType
			}
			writeSingleResponse(w, *repo)
		case http.MethodDelete:
			delete(ms.GitRepositories, repoID)
			w.WriteHeader(http.StatusNoContent)
		}
	})

	ms.Server = httptest.NewServer(mux)
	return ms
}

// handleGitOpsSyncsEndpoint handles GitOps sync API endpoints for a specific environment.
func (ms *MockServer) handleGitOpsSyncsEndpoint(w http.ResponseWriter, r *http.Request, envID string, subpath string) {
	syncs := ms.GitOpsSyncs[envID]
	if syncs == nil {
		syncs = make(map[string]*client.GitOpsSync)
		ms.GitOpsSyncs[envID] = syncs
	}

	// Handle /api/environments/{id}/gitops-syncs (list + create)
	if subpath == "" || subpath == "/" {
		switch r.Method {
		case http.MethodGet:
			syncList := make([]client.GitOpsSync, 0, len(syncs))
			for _, s := range syncs {
				syncList = append(syncList, *s)
			}
			writePaginatedResponse(w, syncList)
		case http.MethodPost:
			var req client.GitOpsSyncCreateRequest
			json.NewDecoder(r.Body).Decode(&req)
			sync := &client.GitOpsSync{
				ID:            "sync-" + req.RepositoryID,
				EnvironmentID: envID,
				RepositoryID:  req.RepositoryID,
				Path:          req.Path,
				Branch:        req.Branch,
				ComposeFile:   req.ComposeFile,
				SyncInterval:  req.SyncInterval,
				AutoSync:      req.AutoSync,
			}
			if sync.Branch == "" {
				sync.Branch = "main"
			}
			if sync.ComposeFile == "" {
				sync.ComposeFile = "docker-compose.yml"
			}
			syncs[sync.ID] = sync
			writeSingleResponse(w, *sync)
		}
		return
	}

	// Handle /api/environments/{id}/gitops-syncs/{syncId}...
	subpath = subpath[1:] // Remove leading /
	syncID := subpath
	action := ""

	// Check for /trigger suffix
	if strings.HasSuffix(subpath, "/trigger") {
		syncID = subpath[:len(subpath)-len("/trigger")]
		action = "trigger"
	}

	sync, exists := syncs[syncID]

	switch {
	case action == "trigger" && r.Method == http.MethodPost:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "sync not found"})
			return
		}
		_ = sync
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "sync not found"})
			return
		}
		writeSingleResponse(w, *sync)
	case r.Method == http.MethodPut:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "sync not found"})
			return
		}
		var req client.GitOpsSyncUpdateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.RepositoryID != "" {
			sync.RepositoryID = req.RepositoryID
		}
		if req.Path != "" {
			sync.Path = req.Path
		}
		if req.Branch != "" {
			sync.Branch = req.Branch
		}
		if req.ComposeFile != "" {
			sync.ComposeFile = req.ComposeFile
		}
		if req.SyncInterval != "" {
			sync.SyncInterval = req.SyncInterval
		}
		if req.AutoSync != nil {
			sync.AutoSync = *req.AutoSync
		}
		writeSingleResponse(w, *sync)
	case r.Method == http.MethodDelete:
		delete(syncs, syncID)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, client.APIError{Message: "not found"})
	}
}

func (ms *MockServer) handleTestEndpoint(w http.ResponseWriter, r *http.Request, envID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if ms.HealthyEnvs[envID] {
		w.WriteHeader(http.StatusOK)
		writeJSON(w, map[string]string{"status": "connected"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, client.APIError{Message: "agent not connected"})
	}
}

func (ms *MockServer) handleProjectsEndpoint(w http.ResponseWriter, r *http.Request, envID string, subpath string) {
	projects := ms.Projects[envID]
	if projects == nil {
		projects = make(map[string]*client.Project)
		ms.Projects[envID] = projects
	}

	// Handle /api/environments/{id}/projects (list)
	if subpath == "" || subpath == "/" {
		projectList := make([]client.Project, 0, len(projects))
		for _, p := range projects {
			projectList = append(projectList, *p)
		}
		writePaginatedResponse(w, projectList)
		return
	}

	// Handle /api/environments/{id}/projects/{projectId}...
	subpath = subpath[1:] // Remove leading /
	var projectID string
	var action string

	// Check for action suffixes
	for _, a := range []string{"/up", "/down", "/redeploy", "/containers"} {
		if idx := len(subpath) - len(a); idx > 0 && subpath[idx:] == a {
			projectID = subpath[:idx]
			action = a[1:]
			break
		}
	}
	if action == "" {
		projectID = subpath
	}

	project, exists := projects[projectID]

	switch {
	case action == "up" && r.Method == http.MethodPost:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "project not found"})
			return
		}
		project.Status = "running"
		w.WriteHeader(http.StatusOK)
	case action == "down" && r.Method == http.MethodPost:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "project not found"})
			return
		}
		project.Status = "stopped"
		w.WriteHeader(http.StatusOK)
	case action == "redeploy" && r.Method == http.MethodPost:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "project not found"})
			return
		}
		project.Status = "running"
		w.WriteHeader(http.StatusOK)
	case action == "containers" && r.Method == http.MethodGet:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "project not found"})
			return
		}
		containers := ms.Containers[envID][projectID]
		if containers == nil {
			containers = []client.ContainerDetail{}
		}
		writePaginatedResponse(w, containers)
	case action == "" && r.Method == http.MethodGet:
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, client.APIError{Message: "project not found"})
			return
		}
		writeSingleResponse(w, *project)
	default:
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, client.APIError{Message: "not found"})
	}
}

// AddProject adds a mock project to an environment.
func (ms *MockServer) AddProject(envID string, project *client.Project) {
	if ms.Projects[envID] == nil {
		ms.Projects[envID] = make(map[string]*client.Project)
	}
	ms.Projects[envID][project.ID] = project
}

// AddContainers adds mock container details for a project.
func (ms *MockServer) AddContainers(envID, projectID string, containers []client.ContainerDetail) {
	if ms.Containers[envID] == nil {
		ms.Containers[envID] = make(map[string][]client.ContainerDetail)
	}
	ms.Containers[envID][projectID] = containers
}

// AddGitOpsSync adds a mock GitOps sync to an environment.
func (ms *MockServer) AddGitOpsSync(envID string, sync *client.GitOpsSync) {
	if ms.GitOpsSyncs[envID] == nil {
		ms.GitOpsSyncs[envID] = make(map[string]*client.GitOpsSync)
	}
	ms.GitOpsSyncs[envID][sync.ID] = sync
}

// handleContainerEndpoint handles individual container lookups.
func (ms *MockServer) handleContainerEndpoint(w http.ResponseWriter, r *http.Request, envID string, containerID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Search through all project containers
	for _, containers := range ms.Containers[envID] {
		for _, c := range containers {
			if c.ID == containerID {
				writeSingleResponse(w, c)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNotFound)
	writeJSON(w, client.APIError{Message: "container not found"})
}

// TestProvider_Schema validates the provider schema is correct.
func TestProvider_Schema(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "arcane" {
  url = "http://localhost:8000"
}
`,
				PlanOnly: true,
			},
		},
	})
}
