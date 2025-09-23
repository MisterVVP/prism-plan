package httpclient

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
	req.Header.Set("Accept-Encoding", "gzip")
	if c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.Bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return resp, err
	}
	if err := decodeJSONBody(resp, out); err != nil {
		return resp, err
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
	req.Header.Set("Accept-Encoding", "gzip")
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
	if err := decodeJSONBody(resp, out); err != nil {
		return resp, err
	}
	return resp, nil
}

func decodeJSONBody(resp *http.Response, out any) error {
	defer resp.Body.Close()

	reader := resp.Body
	var gz *gzip.Reader
	if hasGzipEncoding(resp.Header.Get("Content-Encoding")) {
		var err error
		gz, err = gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		reader = gz
		defer gz.Close()
	}

	if out != nil {
		if err := json.NewDecoder(reader).Decode(out); err != nil {
			return err
		}
	}

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}

	return nil
}

func hasGzipEncoding(header string) bool {
	if header == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "gzip") {
			return true
		}
	}
	return false
}
