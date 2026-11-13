// Package pokemontcg reads cards and their TCGplayer market prices from the
// Pokémon TCG API (https://docs.pokemontcg.io). Only the fields the catalog uses
// are decoded.
package pokemontcg

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL = "https://api.pokemontcg.io/v2"
	maxPageSize    = 250
	// The API fails about 2 out of 3 requests on bad days, with no pattern.
	maxAttempts  = 10
	maxRetryWait = 15 * time.Second
)

type Set struct {
	ID           string `json:"id"` // "sv3pt5"
	Name         string `json:"name"`
	PtcgoCode    string `json:"ptcgoCode"` // "MEW", the code printed on the cards
	PrintedTotal int    `json:"printedTotal"`
}

type Card struct {
	ID        string   `json:"id"` // "sv3pt5-6"
	Name      string   `json:"name"`
	Number    string   `json:"number"` // "6", or "TG01" for some subsets
	Rarity    string   `json:"rarity"`
	Supertype string   `json:"supertype"` // "Pokémon", "Trainer" or "Energy"
	Types     []string `json:"types"`
	Images    struct {
		Small string `json:"small"`
	} `json:"images"`
	TCGPlayer *struct {
		// Keyed by print variant: "normal", "holofoil", "reverseHolofoil"...
		Prices map[string]Price `json:"prices"`
	} `json:"tcgplayer"`
}

// Price is in US dollars. Any field can be missing for cards with few sales.
type Price struct {
	Market *float64 `json:"market"`
	Mid    *float64 `json:"mid"`
}

type Client struct {
	baseURL   string
	apiKey    string
	http      *http.Client
	retryWait time.Duration
}

// NewClient works without an API key, with a lower rate limit (1000 requests a day).
func NewClient(apiKey string) *Client {
	// The API often takes several seconds for a page of 250 cards.
	return &Client{
		baseURL:   defaultBaseURL,
		apiKey:    apiKey,
		http:      &http.Client{Timeout: 90 * time.Second},
		retryWait: 2 * time.Second,
	}
}

func (c *Client) GetSet(ctx context.Context, setID string) (Set, error) {
	var body struct {
		Data Set `json:"data"`
	}
	err := c.get(ctx, "/sets/"+url.PathEscape(setID), nil, &body)
	return body.Data, err
}

// GetCards returns every card of a set, following the pagination.
func (c *Client) GetCards(ctx context.Context, setID string) ([]Card, error) {
	var cards []Card
	for page := 1; ; page++ {
		var body struct {
			Data       []Card `json:"data"`
			TotalCount int    `json:"totalCount"`
		}
		query := url.Values{
			"q":        {"set.id:" + setID},
			"page":     {fmt.Sprint(page)},
			"pageSize": {fmt.Sprint(maxPageSize)},
			"select":   {"id,name,number,rarity,supertype,types,images,tcgplayer"},
		}
		if err := c.get(ctx, "/cards", query, &body); err != nil {
			return nil, err
		}
		cards = append(cards, body.Data...)
		if len(body.Data) == 0 || len(cards) >= body.TotalCount {
			return cards, nil
		}
	}
}

// get retries 429 and 5xx, which the API returns often for a few seconds at a time.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if query != nil {
		endpoint += "?" + query.Encode()
	}

	wait := c.retryWait
	for attempt := 1; ; attempt++ {
		retry, err := c.try(ctx, endpoint, out)
		if err == nil || !retry || attempt == maxAttempts {
			if err != nil {
				return fmt.Errorf("pokemontcg.io %s: %w", path, err)
			}
			return nil
		}
		slog.Warn("pokemontcg.io failed, retrying", "path", path, "error", err, "wait", wait)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait = min(wait*2, maxRetryWait)
	}
}

func (c *Client) try(ctx context.Context, endpoint string, out any) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retry = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return retry, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		// A connection cut mid-body shows up here as truncated JSON.
		return true, err
	}
	return false, nil
}
