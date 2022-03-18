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
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/instanceprotector"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type Client struct {
	server       *url.URL
	requester    *http.Client
	heartbeat    time.Duration
	offlineToken string
	oAuthURL     string
	accessToken  string
	clientId     string
	clientSecret string

	tokenMu sync.RWMutex
}

type ClientConfig struct {
	BaseURL      string
	Heartbeat    time.Duration
	TlsConfig    *tls.Config
	OfflineToken string
	OAuthURL     string
	ClientId     string
	ClientSecret string
	BasePath     string
	ProxyURL     string
}

// Represents the implementation of a job type as defined by the worker API.
type JobImplementation interface {
	Run(ctx context.Context, job *Job) (interface{}, error)
}

var ErrClientRequestJobTimeout = errors.New("Dequeue timed out, retry")

type Job struct {
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
	if conf.ProxyURL != "" {
		proxyURL, err := url.Parse(conf.ProxyURL)
		if err != nil {
			return nil, err
		}

		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			return proxyURL, nil
		}
	}

	if conf.TlsConfig != nil {
		transport.TLSClientConfig = conf.TlsConfig
	}
	requester.Transport = transport

	return &Client{
		server:       server,
		requester:    requester,
		heartbeat:    conf.Heartbeat,
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

	resp, err := c.requester.PostForm(c.oAuthURL, data)
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

func (c *Client) newRequest(ctx context.Context, method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	token := func() string {
		c.tokenMu.RLock()
		defer c.tokenMu.RUnlock()
		return c.accessToken
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
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

func (c *Client) requestJob(ctx context.Context, types []string, arch string) (*Job, error) {
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

	response, err := c.newRequest(ctx, "POST", url.String(), map[string]string{"Content-Type": "application/json"}, &buf)
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

	return &Job{
		client:           c,
		id:               jobId,
		jobType:          jr.Type,
		args:             args,
		dynamicArgs:      dynamicArgs,
		location:         location.String(),
		artifactLocation: artifactLocation.String(),
	}, nil
}

// Regularly ask osbuild-composer if the compose we're currently working on was
// canceled and cancel the job context if it was.
func (job *Job) watch(ctx context.Context, heartbeat time.Duration, cancelRun context.CancelFunc) {
	if heartbeat == 0 {
		return
	}

	for {
		select {
		case <-time.After(heartbeat):
			canceled, err := job.Canceled(ctx)
			if err == nil && canceled {
				cancelRun()
			}
		case <-ctx.Done():
			return
		}
	}
}

func (job *Job) run(ctx context.Context, heartbeat time.Duration, ip *instanceprotector.InstanceProtector, impl JobImplementation) error {
	// request the VM running the job not be shut down until we finish
	ip.Protect()
	defer ip.Unprotect()

	logrus.Infof("Running job '%s' (%s)\n", job.Id(), job.Type())

	watcherCtx, cancelWatcher := context.WithCancel(ctx)
	defer cancelWatcher()
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	go job.watch(watcherCtx, heartbeat, cancelRun)

	result, err := impl.Run(runCtx, job)
	select {
	case <-ctx.Done():
		// The worker is shutting down, don't report a partial result.
		// The job will be requeued by composer once the heartbeat times out.
		logrus.Warnf("Worker interrupted while running job '%s' (%s). Ignoring result", job.Id(), job.Type())
		err = job.interrupt(context.Background())
		if err != nil {
			logrus.Warnf("Error interrupting job '%s' (%s): %v", job.Id(), job.Type(), err)
		}
	case <-runCtx.Done():
		// Composer cancelled the job.
		logrus.Infof("Job '%s' (%s) cancelled remotely. Ignoring result", job.Id(), job.Type())
	default:
		if err == nil {
			logrus.Infof("Job '%s' (%s) finished", job.Id(), job.Type())
		} else {
			logrus.Warnf("Job '%s' (%s) failed: %v", job.Id(), job.Type(), err)
		}
		if result != nil {
			err = job.finish(ctx, result)
			if err != nil {
				logrus.Warnf("Error reporting job result for '%s' (%s): %v", job.Id(), job.Type(), err)
			}
		}
	}

	return nil
}

// Requests and runs 1 job of specified type(s)
// Returning an error here will result in the worker backing off for a while and retrying
func (c *Client) requestAndRunJob(ctx context.Context, ip *instanceprotector.InstanceProtector, arch string, jobImpls map[string]JobImplementation) error {
	acceptedJobTypes := []string{}
	for jt := range jobImpls {
		acceptedJobTypes = append(acceptedJobTypes, jt)
	}

	logrus.Debug("Waiting for a new job...")
	job, err := c.requestJob(ctx, acceptedJobTypes, arch)
	if err != nil {
		if errors.Is(err, ErrClientRequestJobTimeout) {
			logrus.Debugf("Requesting job timed out: %v", err)
			return nil
		} else if errors.Is(err, context.Canceled) {
			logrus.Debugf("Requesting job canceled: %v", err)
		} else {
			logrus.Errorf("Requesting job failed: %v", err)
		}
		return err
	}

	impl, exists := jobImpls[job.Type()]
	if !exists {
		logrus.Errorf("Ignoring job with unknown type %s", job.Type())
		return err
	}

	err = job.run(ctx, c.heartbeat, ip, impl)
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) Start(ctx context.Context, backoffDuration time.Duration, ip *instanceprotector.InstanceProtector, arch string, impls map[string]JobImplementation) {
	for {
		err := client.requestAndRunJob(ctx, ip, arch, impls)
		if errors.Is(err, context.Canceled) {
			return
		} else if err != nil {
			logrus.Warn("Received error from RequestAndRunJob, backing off")
			time.Sleep(backoffDuration)
		}
	}
}

func (j *Job) Id() uuid.UUID {
	return j.id
}

func (j *Job) Type() string {
	return j.jobType
}

func (j *Job) Args(args interface{}) error {
	err := json.Unmarshal(j.args, args)
	if err != nil {
		return fmt.Errorf("error parsing job arguments: %v", err)
	}
	return nil
}

func (j *Job) NDynamicArgs() int {
	return len(j.dynamicArgs)
}

func (j *Job) DynamicArgs(i int, args interface{}) error {
	err := json.Unmarshal(j.dynamicArgs[i], args)
	if err != nil {
		return fmt.Errorf("error parsing job arguments: %v", err)
	}
	return nil
}

func (j *Job) finish(ctx context.Context, result interface{}) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(updateJobRequest{
		Result: result,
	})
	if err != nil {
		panic(err)
	}

	response, err := j.client.newRequest(ctx, "PATCH", j.location, map[string]string{"Content-Type": "application/json"}, &buf)
	if err != nil {
		return fmt.Errorf("error fetching job info: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errorFromResponse(response, "error setting job status")
	}

	return nil
}

func (j *Job) interrupt(ctx context.Context) error {
	response, err := j.client.newRequest(ctx, "DELETE", j.location, map[string]string{}, nil)
	if err != nil {
		return fmt.Errorf("error interrupting job: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errorFromResponse(response, "error interrupting job")
	}

	var jr api.InterruptJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	return nil
}

func (j *Job) Canceled(ctx context.Context) (bool, error) {
	response, err := j.client.newRequest(ctx, "GET", j.location, map[string]string{}, nil)
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

func (j *Job) UploadArtifact(ctx context.Context, name string, reader io.Reader) error {
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

	response, err := j.client.newRequest(ctx, "PUT", loc.String(), map[string]string{"Content-Type": "application/octet-stream"}, reader)
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
