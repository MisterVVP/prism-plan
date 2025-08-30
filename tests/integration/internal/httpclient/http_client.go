package httpclient

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Client wraps http.Client with helpers for JSON requests.
type Client struct {
	BaseURL string
	Bearer  string
	HTTP    *http.Client
}

// New creates a new Client.
func New(baseURL, bearer string) *Client {
	return &Client{BaseURL: baseURL, Bearer: bearer, HTTP: &http.Client{}}
}

// GetJSON issues a GET request and decodes the JSON response.
func (c *Client) GetJSON(path string, out any) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.Bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return resp, err
	}
	if out != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

// PostJSON issues a POST request with a JSON body and decodes the response.
func (c *Client) PostJSON(path string, body, out any) (*http.Response, error) {
	buf := new(bytes.Buffer)
	if body != nil {
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.Bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return resp, err
	}
	if out != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	}
	return resp, nil
}
