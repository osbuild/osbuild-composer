// helper functions for client_test.go
package worker

func (c *Client) InvalidateAccessToken() {
	c.tokenMu.Lock()
	c.accessToken = ""
	c.tokenMu.Unlock()
}
