package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type Client struct {
	server       *url.URL
	requester    *http.Client
	offlineToken string
	oAuthURL     string
	accessToken  string
	clientId     string
	clientSecret string

	tokenMu sync.RWMutex
}

type ClientConfig struct {
	BaseURL      string
	TlsConfig    *tls.Config
	OfflineToken string
	OAuthURL     string
	ClientId     string
	ClientSecret string
	BasePath     string
}

type Job interface {
	Id() uuid.UUID
	Type() string
	Args(args interface{}) error
	DynamicArgs(i int, args interface{}) error
	NDynamicArgs() int
	Update(result interface{}) error
	Canceled() (bool, error)
	UploadArtifact(name string, reader io.Reader) error
}

var ErrClientRequestJobTimeout = errors.New("Dequeue timed out, retry")

type job struct {
	client           *Client
	id               uuid.UUID
	location         string
	artifactLocation string
	jobType          string
	args             json.RawMessage
	dynamicArgs      []json.RawMessage
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

func NewClient(conf ClientConfig) (*Client, error) {
	server, err := url.Parse(conf.BaseURL)
	if err != nil {
		return nil, err
	}

	api.BasePath = conf.BasePath

	server, err = server.Parse(api.BasePath + "/")
	if err != nil {
		panic(err)
	}

	if conf.OAuthURL != "" {
		if conf.ClientId == "" {
			return nil, fmt.Errorf("OAuthURL token url specified but no client id")
		}
		if conf.OfflineToken == "" && conf.ClientSecret == "" {
			return nil, fmt.Errorf("OAuthURL token url specified but no client secret or offline token")
		}
	}

	requester := &http.Client{}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if conf.TlsConfig != nil {
		transport.TLSClientConfig = conf.TlsConfig
	}
	requester.Transport = transport

	return &Client{
		server:       server,
		requester:    requester,
		offlineToken: conf.OfflineToken,
		oAuthURL:     conf.OAuthURL,
		clientId:     conf.ClientId,
		clientSecret: conf.ClientSecret,
	}, nil
}

func NewClientUnix(conf ClientConfig) *Client {
	server, err := url.Parse("http://localhost/")
	if err != nil {
		panic(err)
	}

	api.BasePath = conf.BasePath

	server, err = server.Parse(api.BasePath + "/")
	if err != nil {
		panic(err)
	}

	requester := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", conf.BaseURL)
			},
		},
	}

	return &Client{
		server:    server,
		requester: requester,
	}
}

func (c *Client) refreshAccessToken() error {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	data := url.Values{}
	if c.offlineToken != "" {
		data.Set("grant_type", "refresh_token")
		data.Set("client_id", c.clientId)
		data.Set("refresh_token", c.offlineToken)
	}
	if c.clientSecret != "" {
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", c.clientId)
		data.Set("client_secret", c.clientSecret)
	}

	resp, err := http.PostForm(c.oAuthURL, data)
	if err != nil {
		return err
	}

	var tr tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tr)
	if err != nil {
		return err
	}

	c.accessToken = tr.AccessToken
	return nil
}

func (c *Client) NewRequest(method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	token := func() string {
		c.tokenMu.RLock()
		defer c.tokenMu.RUnlock()
		return c.accessToken
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if c.oAuthURL != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token()))
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.requester.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized && c.oAuthURL != "" {
		err = c.refreshAccessToken()
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token()))
		resp, err = c.requester.Do(req)
	}
	return resp, err
}

func (c *Client) RequestJob(types []string, arch string) (Job, error) {
	url, err := c.server.Parse("jobs")
	if err != nil {
		// This only happens when "jobs" cannot be parsed.
		panic(err)
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(api.RequestJobJSONRequestBody{
		Types: types,
		Arch:  arch,
	})
	if err != nil {
		panic(err)
	}

	response, err := c.NewRequest("POST", url.String(), map[string]string{"Content-Type": "application/json"}, &buf)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		return nil, ErrClientRequestJobTimeout
	}
	if response.StatusCode != http.StatusCreated {
		return nil, errorFromResponse(response, "error requesting job")
	}

	var jr api.RequestJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	location, err := c.server.Parse(jr.Location)
	if err != nil {
		return nil, fmt.Errorf("error parsing location url in response: %v", err)
	}

	artifactLocation, err := c.server.Parse(jr.ArtifactLocation)
	if err != nil {
		return nil, fmt.Errorf("error parsing artifact location url in response: %v", err)
	}

	jobId, err := uuid.Parse(jr.Id)
	if err != nil {
		return nil, fmt.Errorf("error parsing job id in response: %v", err)
	}

	args := json.RawMessage{}
	if jr.Args != nil {
		args = *jr.Args
	}
	dynamicArgs := []json.RawMessage{}
	if jr.DynamicArgs != nil {
		dynamicArgs = *jr.DynamicArgs
	}

	return &job{
		client:           c,
		id:               jobId,
		jobType:          jr.Type,
		args:             args,
		dynamicArgs:      dynamicArgs,
		location:         location.String(),
		artifactLocation: artifactLocation.String(),
	}, nil
}

func (j *job) Id() uuid.UUID {
	return j.id
}

func (j *job) Type() string {
	return j.jobType
}

func (j *job) Args(args interface{}) error {
	err := json.Unmarshal(j.args, args)
	if err != nil {
		return fmt.Errorf("error parsing job arguments: %v", err)
	}
	return nil
}

func (j *job) NDynamicArgs() int {
	return len(j.dynamicArgs)
}

func (j *job) DynamicArgs(i int, args interface{}) error {
	err := json.Unmarshal(j.dynamicArgs[i], args)
	if err != nil {
		return fmt.Errorf("error parsing job arguments: %v", err)
	}
	return nil
}

func (j *job) Update(result interface{}) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(updateJobRequest{
		Result: result,
	})
	if err != nil {
		panic(err)
	}

	response, err := j.client.NewRequest("PATCH", j.location, map[string]string{"Content-Type": "application/json"}, &buf)
	if err != nil {
		return fmt.Errorf("error fetching job info: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errorFromResponse(response, "error setting job status")
	}

	return nil
}

func (j *job) Canceled() (bool, error) {
	response, err := j.client.NewRequest("GET", j.location, map[string]string{}, nil)
	if err != nil {
		return false, fmt.Errorf("error fetching job info: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false, errorFromResponse(response, "error fetching job info")
	}

	var jr api.GetJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return false, fmt.Errorf("error parsing reponse: %v", err)
	}

	return jr.Canceled, nil
}

func (j *job) UploadArtifact(name string, reader io.Reader) error {
	if j.artifactLocation == "" {
		return fmt.Errorf("server does not accept artifacts for this job")
	}

	loc, err := url.Parse(j.artifactLocation)
	if err != nil {
		return fmt.Errorf("error parsing job location: %v", err)
	}

	loc, err = loc.Parse(url.PathEscape(name))
	if err != nil {
		panic(err)
	}

	response, err := j.client.NewRequest("PUT", loc.String(), map[string]string{"Content-Type": "application/octet-stream"}, reader)
	if err != nil {
		return fmt.Errorf("error uploading artifact: %v", err)
	}

	if response.StatusCode != 200 {
		return errorFromResponse(response, "error uploading artifact")
	}

	return nil
}

// Parses an api.Error from a response and returns it as a golang error. Other
// errors, such failing to parse the response, are returned as golang error as
// well. If client code expects an error, it gets one.
func errorFromResponse(response *http.Response, message string) error {
	var e api.Error
	err := json.NewDecoder(response.Body).Decode(&e)
	if err != nil {
		return fmt.Errorf("failed to parse error response: %v", err)
	}
	return fmt.Errorf("%v: %v — %s (%v)", message, response.StatusCode, e.Reason, e.Code)
}
