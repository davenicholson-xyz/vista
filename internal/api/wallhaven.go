package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const baseURL = "https://wallhaven.cc/api/v1/search"

type Thumbs struct {
	Large    string `json:"large"`
	Original string `json:"original"`
	Small    string `json:"small"`
}

type Wallpaper struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Path       string `json:"path"`
	Resolution string `json:"resolution"`
	Thumbs     Thumbs `json:"thumbs"`
}

type Meta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	Total       int `json:"total"`
}

type searchResponse struct {
	Data []Wallpaper `json:"data"`
	Meta Meta        `json:"meta"`
}

// SearchOptions controls what the API returns.
// Sorting values: relevance, date_added, random, views, favorites, toplist, hot.
type SearchOptions struct {
	Query   string
	Sorting string
}

type Client struct {
	APIKey        string
	Username      string
	Purity        string
	Categories    string
	MinResolution string
}

func (c *Client) SearchPage(opts SearchOptions, page int) ([]Wallpaper, Meta, error) {
	params := url.Values{}
	if opts.Query != "" {
		params.Set("q", opts.Query)
	}
	if opts.Sorting != "" {
		params.Set("sorting", opts.Sorting)
	}
	params.Set("page", fmt.Sprintf("%d", page))
	if c.Purity != "" {
		params.Set("purity", c.Purity)
	}
	if c.Categories != "" {
		params.Set("categories", c.Categories)
	}
	if c.MinResolution != "" {
		params.Set("atleast", c.MinResolution)
	}
	if c.APIKey != "" {
		params.Set("apikey", c.APIKey)
	}

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("creating request: %w", err)
	}
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, Meta{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, Meta{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, Meta{}, fmt.Errorf("decoding response: %w", err)
	}

	return result.Data, result.Meta, nil
}
