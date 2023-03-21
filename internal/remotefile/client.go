package remotefile

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	client *http.Client
}

func NewClient() *Client {
	return &Client{
		client: &http.Client{},
	}
}

func (c *Client) makeRequest(u *url.URL) ([]byte, error) {
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	output, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (c *Client) validateURL(u string) (*url.URL, error) {
	if u == "" {
		return nil, fmt.Errorf("File resolver: url is required")
	}
	parsedURL, err := url.ParseRequestURI(u)
	if err != nil {
		return nil, fmt.Errorf("File resolver: invalid url %s", u)
	}
	return parsedURL, nil
}

// resolve and return the contents of a remote file
// which can be used later, in the pipeline
func (c *Client) Resolve(u string) ([]byte, error) {
	parsedURL, err := c.validateURL(u)
	if err != nil {
		return nil, err
	}

	return c.makeRequest(parsedURL)
}
