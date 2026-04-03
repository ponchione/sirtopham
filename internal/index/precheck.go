package index

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ponchione/sirtopham/internal/config"
)

const defaultDescriberBaseURL = "http://localhost:8080"

var describerBaseURL = defaultDescriberBaseURL

type servicePrecheck struct {
	name          string
	baseURL       string
	healthPath    string
	modelsPath    string
	requireModels bool
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func runIndexPrecheck(ctx context.Context, cfg *config.Config) error {
	client := &http.Client{Timeout: 3 * time.Second}
	checks := []servicePrecheck{
		{
			name:          "qwen-coder",
			baseURL:       strings.TrimRight(describerBaseURL, "/"),
			healthPath:    "/health",
			modelsPath:    "/v1/models",
			requireModels: true,
		},
		{
			name:          "nomic-embed",
			baseURL:       strings.TrimRight(cfg.Embedding.BaseURL, "/"),
			healthPath:    "/health",
			modelsPath:    "/v1/models",
			requireModels: true,
		},
	}
	for _, check := range checks {
		if err := checkService(ctx, client, check); err != nil {
			return err
		}
	}
	return nil
}

func checkService(ctx context.Context, client *http.Client, check servicePrecheck) error {
	if strings.TrimSpace(check.baseURL) == "" {
		return fmt.Errorf("index precheck: %s base URL is empty", check.name)
	}

	if err := requireHTTP200(ctx, client, check.name, check.baseURL+check.healthPath); err != nil {
		return err
	}
	if check.requireModels {
		if err := requireModelsEndpoint(ctx, client, check.name, check.baseURL+check.modelsPath); err != nil {
			return err
		}
	}
	return nil
}

func requireHTTP200(ctx context.Context, client *http.Client, serviceName, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("index precheck: build %s request: %w", serviceName, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("index precheck: required service %s is not reachable at %s: %w", serviceName, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index precheck: required service %s is unhealthy at %s: HTTP %d", serviceName, url, resp.StatusCode)
	}
	return nil
}

func requireModelsEndpoint(ctx context.Context, client *http.Client, serviceName, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("index precheck: build %s models request: %w", serviceName, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("index precheck: required service %s models endpoint is not reachable at %s: %w", serviceName, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index precheck: required service %s models endpoint is unhealthy at %s: HTTP %d", serviceName, url, resp.StatusCode)
	}
	var models modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return fmt.Errorf("index precheck: decode %s models response: %w", serviceName, err)
	}
	if len(models.Data) == 0 {
		return fmt.Errorf("index precheck: required service %s returned no models from %s", serviceName, url)
	}
	return nil
}
