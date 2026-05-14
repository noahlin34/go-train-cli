package metrolinx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) Get(ctx context.Context, path string, v any) error {
	if c.apiKey == "" {
		return fmt.Errorf("missing GO API key: set GO_API_KEY or pass --api-key")
	}
	u, err := url.Parse(c.baseURL + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("key", c.apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("metrolinx HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) Stops(ctx context.Context) ([]Stop, Metadata, error) {
	var resp struct {
		Metadata Metadata `json:"Metadata"`
		Stations struct {
			Station []Stop `json:"Station"`
		} `json:"Stations"`
	}
	err := c.Get(ctx, "api/V1/Stop/All", &resp)
	if err != nil {
		return nil, resp.Metadata, err
	}
	return resp.Stations.Station, resp.Metadata, requireOK(resp.Metadata)
}

func (c *Client) NextService(ctx context.Context, stopCode string) (json.RawMessage, Metadata, error) {
	var resp struct {
		Metadata    Metadata        `json:"Metadata"`
		NextService json.RawMessage `json:"NextService"`
	}
	err := c.Get(ctx, "api/V1/Stop/NextService/"+url.PathEscape(strings.ToUpper(stopCode)), &resp)
	if err != nil {
		return nil, resp.Metadata, err
	}
	return resp.NextService, resp.Metadata, requireOK(resp.Metadata)
}

func (c *Client) TrainTrips(ctx context.Context) ([]TrainTrip, Metadata, error) {
	var resp struct {
		Metadata Metadata `json:"Metadata"`
		Trips    struct {
			Trip json.RawMessage `json:"Trip"`
		} `json:"Trips"`
	}
	err := c.Get(ctx, "api/V1/ServiceataGlance/Trains/All", &resp)
	if err != nil {
		return nil, resp.Metadata, err
	}
	trips, err := oneOrMany[TrainTrip](resp.Trips.Trip)
	if err != nil {
		return nil, resp.Metadata, err
	}
	return trips, resp.Metadata, requireOK(resp.Metadata)
}

func (c *Client) ServiceAlerts(ctx context.Context) ([]AlertMessage, Metadata, error) {
	var resp struct {
		Metadata Metadata `json:"Metadata"`
		Messages struct {
			Message json.RawMessage `json:"Message"`
		} `json:"Messages"`
	}
	err := c.Get(ctx, "api/V1/ServiceUpdate/ServiceAlert/All", &resp)
	if err != nil {
		return nil, resp.Metadata, err
	}
	messages, err := oneOrMany[AlertMessage](resp.Messages.Message)
	if err != nil {
		return nil, resp.Metadata, err
	}
	return messages, resp.Metadata, requireOK(resp.Metadata)
}
