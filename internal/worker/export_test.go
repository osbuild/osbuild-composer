package worker

import (
	"net/url"
)

var (
	NewServerURL = newServerURL
)

func (c *Client) SetServerURL(client *Client, url *url.URL) {
	c.serverURL = url
}

func (c *Client) Endpoint(endpoint string) string {
	return c.endpoint(endpoint)
}
