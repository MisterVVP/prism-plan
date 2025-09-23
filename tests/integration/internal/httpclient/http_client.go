package httpclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
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
	req.Header.Set("Accept", "application/json")
	if c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.Bearer)
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
	var reader io.Reader
	if body != nil {
		jsonBuf := new(bytes.Buffer)
		encoder := json.NewEncoder(jsonBuf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(body); err != nil {
			return nil, err
		}

		var gzBuf bytes.Buffer
		gzWriter, err := gzip.NewWriterLevel(&gzBuf, gzip.BestSpeed)
		if err != nil {
			return nil, err
		}
		if _, err := gzWriter.Write(jsonBuf.Bytes()); err != nil {
			gzWriter.Close()
			return nil, err
		}
		if err := gzWriter.Close(); err != nil {
			return nil, err
		}
		reader = bytes.NewReader(gzBuf.Bytes())
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if reader != nil {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.Bearer)
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
