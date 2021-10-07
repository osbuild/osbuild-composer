package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type bearerToken struct {
	AccessToken     string `json:"access_token"`
	ValidForSeconds int    `json:"expires_in"`
}

type Client struct {
	server           *url.URL
	requester        *http.Client
	offlineToken     *string
	oAuthURL         *string
	lastTokenRefresh *time.Time
	bearerToken      *bearerToken

	tokenMu *sync.Mutex
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

type job struct {
	client           *Client
	id               uuid.UUID
	location         string
	artifactLocation string
	jobType          string
	args             json.RawMessage
	dynamicArgs      []json.RawMessage
}

func NewClient(baseURL string, conf *tls.Config, offlineToken, oAuthURL *string, basePath string) (*Client, error) {
	server, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	api.BasePath = basePath

	server, err = server.Parse(api.BasePath + "/")
	if err != nil {
		panic(err)
	}

	requester := &http.Client{}
	if conf != nil {
		requester.Transport = &http.Transport{
			TLSClientConfig: conf,
		}
	}

	return &Client{server, requester, offlineToken, oAuthURL, nil, nil, &sync.Mutex{}}, nil
}

func NewClientUnix(path string, basePath string) *Client {
	server, err := url.Parse("http://localhost/")
	if err != nil {
		panic(err)
	}

	api.BasePath = basePath

	server, err = server.Parse(api.BasePath + "/")
	if err != nil {
		panic(err)
	}

	requester := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	return &Client{server, requester, nil, nil, nil, nil, nil}
}

// Note: Only call this function with Client.tokenMu locked!
func (c *Client) refreshBearerToken() error {
	if c.offlineToken == nil || c.oAuthURL == nil {
		return fmt.Errorf("No offline token or oauth url available")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", "rhsm-api")
	data.Set("refresh_token", *c.offlineToken)

	t := time.Now()
	resp, err := http.PostForm(*c.oAuthURL, data)
	if err != nil {
		return err
	}

	var bt bearerToken
	err = json.NewDecoder(resp.Body).Decode(&bt)
	if err != nil {
		return err
	}

	c.bearerToken = &bt
	c.lastTokenRefresh = &t
	return nil
}

func (c *Client) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// If we're using OAUTH, add the Bearer token
	if c.offlineToken != nil {
		// make sure we have a valid token
		var d time.Duration
		c.tokenMu.Lock()
		defer c.tokenMu.Unlock()
		if c.lastTokenRefresh != nil {
			d = time.Since(*c.lastTokenRefresh)
		}
		if c.bearerToken == nil || d.Seconds() >= (float64(c.bearerToken.ValidForSeconds)*0.8) {
			err = c.refreshBearerToken()
			if err != nil {
				return nil, err
			}
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken.AccessToken))
	}
	return req, nil
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

	req, err := c.NewRequest("POST", url.String(), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	response, err := c.requester.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting job: %v", err)
	}
	defer response.Body.Close()

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

	req, err := j.client.NewRequest("PATCH", j.location, &buf)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Content-Type", "application/json")

	response, err := j.client.requester.Do(req)
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
	req, err := j.client.NewRequest("GET", j.location, nil)
	if err != nil {
		return false, err
	}

	response, err := j.client.requester.Do(req)
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

	req, err := j.client.NewRequest("PUT", loc.String(), reader)
	if err != nil {
		return fmt.Errorf("cannot create request: %v", err)
	}

	req.Header.Add("Content-Type", "application/octet-stream")

	response, err := j.client.requester.Do(req)
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
