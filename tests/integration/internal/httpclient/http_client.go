package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// Client wraps http.Client with helpers for JSON requests.
type Client struct {
	BaseURL      string
	Bearer       string
	FunctionsKey string
	HTTP         *http.Client
}

// New creates a new Client.
func New(baseURL, bearer, functionsKey string) *Client {
	return &Client{BaseURL: baseURL, Bearer: bearer, FunctionsKey: functionsKey, HTTP: &http.Client{}}
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
	if c.FunctionsKey != "" {
		req.Header.Set("x-functions-key", c.FunctionsKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
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
	if c.FunctionsKey != "" {
		req.Header.Set("x-functions-key", c.FunctionsKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return resp, nil
}
