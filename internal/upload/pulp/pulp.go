package pulp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/osbuild/pulp-client/pulpclient"
	"github.com/sirupsen/logrus"
)

type Client struct {
	client *pulpclient.APIClient
	ctx    context.Context
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewClientFromFile(url, path string) (*Client, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	data, err := io.ReadAll(fp)
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return NewClient(url, &creds), nil
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

// readBody returns the body of a response as a string and ignores
// errors. Useful for returning details from failed requests.
func readBody(r *http.Response) string {
	if r == nil {
		return ""
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	return string(b)
}

// UploadFile uploads the file at the given path and returns the href of the
// new artifact.
func (cl *Client) UploadFile(path string) (string, error) {
	fp, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fp.Close()
	create := cl.client.ArtifactsAPI.ArtifactsCreate(cl.ctx).File(fp)
	res, resp, err := create.Execute()
	if err != nil {
		return "", fmt.Errorf("failed to upload file %q: %s (%s)", path, err.Error(), readBody(resp))
	}

	return res.GetPulpHref(), nil
}

// GetOSTreeRepositoryByName returns the href for a repository based on its name.
// Returns an empty string without an error if the repository does not exist.
func (cl *Client) GetOSTreeRepositoryByName(name string) (string, error) {
	list, resp, err := cl.client.RepositoriesOstreeAPI.RepositoriesOstreeOstreeList(cl.ctx).Name(name).Execute()
	if err != nil {
		return "", fmt.Errorf("repository list request returned an error: %s (%s)", err.Error(), readBody(resp))
	}

	if list.GetCount() == 0 {
		logrus.Infof("no repository named %s was found", name)
		return "", nil
	}

	if list.GetCount() > 1 {
		return "", fmt.Errorf("more than one repository named %s was found: %s (%s)", name, err.Error(), readBody(resp))
	}

	results := list.GetResults()
	repo := results[0]

	repoHref := repo.GetPulpHref()
	logrus.Infof("found repository %s: %s", name, repoHref)
	return repoHref, nil
}

// ListOSTreeRepositories returns a map (repository name -> pulp href) of
// existing ostree repositories.
func (cl *Client) ListOSTreeRepositories() (map[string]string, error) {
	list, resp, err := cl.client.RepositoriesOstreeAPI.RepositoriesOstreeOstreeList(cl.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("repository list request returned an error: %s (%s)", err.Error(), readBody(resp))
	}

	repos := make(map[string]string, list.GetCount())
	for _, repo := range list.GetResults() {
		name := repo.Name
		href := repo.GetPulpHref()
		repos[name] = href
	}

	return repos, nil
}

// CreateOSTreeRepository creates a new ostree repository with a name and description
// and returns the pulp href.
func (cl *Client) CreateOSTreeRepository(name, description string) (string, error) {
	req := cl.client.RepositoriesOstreeAPI.RepositoriesOstreeOstreeCreate(cl.ctx)
	repo := pulpclient.OstreeOstreeRepository{
		Name: name,
	}
	if description != "" {
		repo.Description = *pulpclient.NewNullableString(&description)
	}
	req = req.OstreeOstreeRepository(repo)
	result, resp, err := req.Execute()
	if err != nil {
		return "", fmt.Errorf("repository creation failed: %s (%s)", err.Error(), readBody(resp))
	}

	return result.GetPulpHref(), nil
}

// ImportCommit imports a commit that has already been uploaded to a given
// repository. The commitHref must reference a commit tarball artifact. This
// task is asynchronous. The returned value is the href for the import task.
func (cl *Client) ImportCommit(commitHref, repoHref string) (string, error) {
	req := cl.client.RepositoriesOstreeAPI.RepositoriesOstreeOstreeImportAll(cl.ctx, repoHref)
	importOptions := *pulpclient.NewOstreeImportAll(commitHref, "repo") // our commit archives always use the repo name "repo"

	result, resp, err := req.OstreeImportAll(importOptions).Execute()
	if err != nil {
		return "", fmt.Errorf("ostree commit import failed: %s (%s)", err.Error(), readBody(resp))
	}

	return result.Task, nil
}

// Distribute makes an ostree repository available for download. This task is
// asynchronous. The returned value is the href for the distribute task.
func (cl *Client) DistributeOSTreeRepo(basePath, name, repoHref string) (string, error) {
	dist := *pulpclient.NewOstreeOstreeDistribution(basePath, name)
	dist.SetRepository(repoHref)
	res, resp, err := cl.client.DistributionsOstreeAPI.DistributionsOstreeOstreeCreate(cl.ctx).OstreeOstreeDistribution(dist).Execute()
	if err != nil {
		return "", fmt.Errorf("error distributing ostree repository: %s (%s)", err.Error(), readBody(resp))
	}

	return res.Task, nil
}

// GetDistributionURLForOSTreeRepo returns the basepath for an ostree
// distribution of a given repository. Returns an empty string without an error
// if the no distribution for the repo exists.
func (cl *Client) GetDistributionURLForOSTreeRepo(repoHref string) (string, error) {
	list, resp, err := cl.client.DistributionsOstreeAPI.DistributionsOstreeOstreeList(cl.ctx).Repository(repoHref).Execute()

	if list.GetCount() == 0 {
		logrus.Infof("no distribution for repo %s was found: %s", repoHref, readBody(resp))
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("error looking up distribution for repo %s: %s (%s)", repoHref, err.Error(), readBody(resp))
	}

	results := list.GetResults()
	// if there's more than one distribution, return the first one
	dist := results[0]

	return dist.GetBaseUrl(), nil
}

type TaskState string

const (
	TASK_WAITING   TaskState = "waiting"
	TASK_SKIPPED   TaskState = "skipped"
	TASK_RUNNING   TaskState = "running"
	TASK_COMPLETED TaskState = "completed"
	TASK_FAILED    TaskState = "failed"
	TASK_CANCELED  TaskState = "canceled"
	TASK_CANCELING TaskState = "canceling"
)

// TaskState returns the state of a given task.
func (cl *Client) TaskState(task string) (TaskState, error) {
	res, resp, err := cl.client.TasksAPI.TasksRead(cl.ctx, task).Execute()
	if err != nil {
		return "", fmt.Errorf("error reading task %s: %s (%s)", task, err.Error(), readBody(resp))
	}

	state := res.GetState()
	if state == "" {
		return "", fmt.Errorf("got empty task state for %s", task)
	}

	return TaskState(state), nil
}

// TaskWaitingOrRunning returns true if the given task is in the running state. Errors
// are ignored and return false.
func (cl *Client) TaskWaitingOrRunning(task string) bool {
	state, err := cl.TaskState(task)
	if err != nil {
		// log the error and return false
		logrus.Errorf("failed to get task state: %s", err.Error())
		return false
	}
	return state == TASK_RUNNING || state == TASK_WAITING
}

// UploadAndDistributeCommit uploads a commit, creates a repository if
// necessary, imports the commit to the repository, and distributes the
// repository.
func (cl *Client) UploadAndDistributeCommit(archivePath, repoName, basePath string) (string, error) {
	// Check for the repository before uploading the commit:
	// If the repository needs to be created but the basePath is empty, we
	// should fail before uploading the commit.
	logrus.Infof("checking if repository %q already exists", repoName)
	repoHref, err := cl.GetOSTreeRepositoryByName(repoName)
	if err != nil {
		return "", err
	}

	if repoHref == "" && basePath == "" {
		return "", fmt.Errorf("repository %q does not exist and needs to be created, but no basepath for distribution was provided", repoName)
	}

	// Upload the file before creating the repository (if we need to create it)
	// in case it fails. We don't want to have an empty repository if the
	// commit upload fails.
	logrus.Infof("uploading ostree commit to pulp")
	fileHref, err := cl.UploadFile(archivePath)
	if err != nil {
		return "", err
	}

	if repoHref == "" {
		// repository does not exist: create it and distribute
		logrus.Infof("repository not found - creating repository %q", repoName)
		href, err := cl.CreateOSTreeRepository(repoName, "")
		if err != nil {
			return "", err
		}

		repoHref = href
		logrus.Infof("created repository %q (%s)", repoName, repoHref)
		logrus.Infof("creating distribution at %q", basePath)
		if _, err := cl.DistributeOSTreeRepo(basePath, repoName, repoHref); err != nil {
			return "", err
		}
	}

	logrus.Infof("importing commit %q to repo %q", fileHref, repoHref)
	if _, err := cl.ImportCommit(fileHref, repoHref); err != nil {
		return "", err
	}

	repoURL, err := cl.GetDistributionURLForOSTreeRepo(repoHref)
	if err != nil {
		return "", err
	}
	logrus.Infof("repository url: %s", repoURL)

	return repoURL, nil
}
