package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"xray-ip-limiter/internal/usecase"
)

var _ usecase.RemnawaveClient = (*RemnawaveHTTPClient)(nil)

type remnawaveUser struct {
	ID              int    `json:"ID"`
	Username        string `json:"username"`
	Status          string `json:"status"`
	HwidDeviceLimit *int   `json:"hwidDeviceLimit"`
}

type remnawaveResponse struct {
	Response struct {
		Users []remnawaveUser `json:"users"`
		Total int             `json:"total"`
	} `json:"response"`
}

type CookiesConfig struct {
	Enabled bool
	Name    string
	Value   string
}

type RemnawaveConfig struct {
	URL           string
	ApiKey        string
	DefaultLimit  int
	EGamesCookies CookiesConfig
}

type RemnawaveHTTPClient struct {
	baseURL              string
	apiKey               string
	defaultLimit         int
	eGamesCookiesEnabled bool
	eGamesCookiesName    string
	eGamesCookiesValue   string
	httpClient           *http.Client
}

func NewRemnawaveClient(cfg RemnawaveConfig) *RemnawaveHTTPClient {
	return &RemnawaveHTTPClient{
		baseURL:              cfg.URL,
		apiKey:               cfg.ApiKey,
		defaultLimit:         cfg.DefaultLimit,
		eGamesCookiesEnabled: cfg.EGamesCookies.Enabled,
		eGamesCookiesName:    cfg.EGamesCookies.Name,
		eGamesCookiesValue:   cfg.EGamesCookies.Value,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *RemnawaveHTTPClient) FetchUsersLimits(ctx context.Context) (map[int]int, error) {
	allLimits := make(map[int]int)
	pageSize, currentStart := 100, 0

	for {
		url := fmt.Sprintf("%s/api/users?start=%d&size=%d", c.baseURL, currentStart, pageSize)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		if c.eGamesCookiesEnabled {
			cookie := http.Cookie{
				Name:  c.eGamesCookiesName,
				Value: c.eGamesCookiesValue,
			}

			req.AddCookie(&cookie)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("api returned error status: %w", err)
		}

		var data remnawaveResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode json: %w", err)
		}
		resp.Body.Close()

		for _, user := range data.Response.Users {
			if user.Status == "ACTIVE" {
				limit := c.defaultLimit
				if user.HwidDeviceLimit != nil {
					limit = *user.HwidDeviceLimit
				}
				allLimits[user.ID] = limit

			}
		}

		if len(data.Response.Users) < pageSize || len(allLimits) >= data.Response.Total {
			break
		}

		currentStart += pageSize
	}
	return allLimits, nil
}
