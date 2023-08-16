package pulp

import (
	"context"
	"net/http"

	"github.com/osbuild/pulp-client/pulpclient"
)

type Client struct {
	client *pulpclient.APIClient
	ctx    context.Context
}

type Credentials struct {
	Username string
	Password string
}

func NewClient(url string, creds *Credentials) *Client {
	ctx := context.WithValue(context.Background(), pulpclient.ContextServerIndex, 0)
	transport := &http.Transport{}
	httpClient := http.Client{Transport: transport}

	pulpConfig := pulpclient.NewConfiguration()
	pulpConfig.HTTPClient = &httpClient
	pulpConfig.Servers = pulpclient.ServerConfigurations{pulpclient.ServerConfiguration{
		URL: url,
	}}
	client := pulpclient.NewAPIClient(pulpConfig)

	if creds != nil {
		ctx = context.WithValue(ctx, pulpclient.ContextBasicAuth, pulpclient.BasicAuth{
			UserName: creds.Username,
			Password: creds.Password,
		})
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}
}
