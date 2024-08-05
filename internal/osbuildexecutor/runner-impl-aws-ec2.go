package osbuildexecutor

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

type awsEC2Executor struct {
	iamProfile      string
	keyName         string
	cloudWatchGroup string
	hostname        string
	tmpDir          string
}

func prepareSources(manifest []byte, store string, extraEnv []string, result bool, errorWriter io.Writer) error {
	hostExecutor := NewHostExecutor()
	opts := &OsbuildOpts{
		StoreDir: store,
		ExtraEnv: extraEnv,
		Result:   result,
	}
	_, err := hostExecutor.RunOSBuild(manifest, opts, errorWriter)
	return err
}

// TODO extract this, also used in the osbuild-worker-executor unit
// tests.
func waitForSI(ctx context.Context, host string) bool {
	client := http.Client{
		Timeout: time.Second * 1,
	}

	for {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/", host))
		if err != nil {
			logrus.Debugf("Waiting for secure instance continues: %v", err)
		}
		if resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				return true
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				logrus.Warningf("Unable to read body waiting for secure instance: %v", err)
			}
			logrus.Debugf("Waiting for secure instance continues: %s", body)
		}
		select {
		case <-ctx.Done():
			logrus.Error("Timeout waiting for secure instance to spin up")
			return false
		default:
			time.Sleep(time.Second)
			continue
		}
	}
}

func writeInputArchive(cacheDir, store string, exports []string, manifestData []byte) (string, error) {
	archive := filepath.Join(cacheDir, "input.tar")
	control := filepath.Join(cacheDir, "control.json")
	manifest := filepath.Join(cacheDir, "manifest.json")

	controlData := struct {
		Exports []string `json:"exports"`
	}{
		Exports: exports,
	}
	controlDataBytes, err := json.Marshal(controlData)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(control, controlDataBytes, 0600)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(manifest, manifestData, 0600)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("tar",
		"-C",
		cacheDir,
		"-cf",
		archive,
		filepath.Base(control),
		filepath.Base(manifest),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("Unable to create input tar: %w, %s", err, output)
	}
	// Separate tar call, as we need to switch to the store directory.
	/* #nosec G204 */
	cmd = exec.Command("tar",
		"-C",
		filepath.Dir(store),
		"-rf",
		archive,
		filepath.Base(store),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("Unable to create input tar: %w, %s", err, output)
	}

	return archive, nil
}

func handleBuild(inputArchive, host string) (*osbuild.Result, error) {
	client := http.Client{
		Timeout: time.Minute * 60,
	}
	inputFile, err := os.Open(inputArchive)
	if err != nil {
		return nil, fmt.Errorf("Unable to open inputArchive (%s): %w", inputArchive, err)
	}
	defer inputFile.Close()

	resp, err := client.Post(fmt.Sprintf("%s/api/v1/build", host), "application/x-tar", inputFile)
	if err != nil {
		return nil, fmt.Errorf("Unable to request build from executor instance: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Unable to read body waiting for build to run: %w,  http status: %d", err, resp.StatusCode)
		}
		return nil, fmt.Errorf("Something went wrong during executor build: http status: %v, %d, %s", err, resp.StatusCode, body)
	}

	var osbuildResult osbuild.Result

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read response body: %w", err)
	}

	err = json.Unmarshal(body, &osbuildResult)
	if err != nil {
		return nil, fmt.Errorf("Unable to decode response body %q into osbuild result: %w", body, err)
	}

	return &osbuildResult, nil
}

func fetchOutputArchive(cacheDir, host string) (string, error) {
	client := http.Client{
		Timeout: time.Minute * 30,
	}

	resp, err := client.Get(fmt.Sprintf("%s/api/v1/result/output.tar", host))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("cannot fetch output archive: %w, http status: %d", err, resp.StatusCode)
		}
		return "", fmt.Errorf("cannot fetch output archive: %w, http status: %d, body: %s", err, resp.StatusCode, body)
	}
	file, err := os.Create(filepath.Join(cacheDir, "output.tar"))
	if err != nil {
		return "", fmt.Errorf("Unable to write executor result tarball: %w", err)
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("Unable to write executor result tarball: %w", err)
	}
	return file.Name(), nil
}

func validateOutputArchive(outputTarPath string) error {
	f, err := os.Open(outputTarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// check for directory traversal attacks
		if filepath.Clean(hdr.Name) != strings.TrimSuffix(hdr.Name, "/") {
			return fmt.Errorf("name %q not clean, got %q after cleaning", hdr.Name, filepath.Clean(hdr.Name))
		}
		if strings.HasPrefix(filepath.Clean(hdr.Name), "/") {
			return fmt.Errorf("name %q must not start with an absolute path", hdr.Name)
		}
		// protect against someone smuggling in eg. device files
		// XXX: should we support symlinks here?
		if !slices.Contains([]byte{tar.TypeReg, tar.TypeDir, tar.TypeGNUSparse}, hdr.Typeflag) {
			return fmt.Errorf("name %q must be a file/dir, is header type %q", hdr.Name, hdr.Typeflag)
		}
		// protect against executables, this implicitly protects
		// against suid/sgid (XXX: or should we also check that?)
		if hdr.Typeflag == tar.TypeReg && hdr.Mode&0111 != 0 {
			return fmt.Errorf("name %q must not be executable (is mode 0%o)", hdr.Name, hdr.Mode)
		}
	}

	return nil
}

func extractOutputArchive(outputDirectory, outputTar string) error {
	// validate against directory traversal attacks
	if err := validateOutputArchive(outputTar); err != nil {
		return fmt.Errorf("unable to validate output tar: %w", err)
	}

	cmd := exec.Command("tar",
		"--strip-components=1",
		"-C",
		outputDirectory,
		"-Sxf",
		outputTar,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Unable to create input tar: %w, %s", err, output)
	}
	return nil

}

func (ec2e *awsEC2Executor) RunOSBuild(manifest []byte, opts *OsbuildOpts, errorWriter io.Writer) (*osbuild.Result, error) {
	if opts == nil {
		opts = &OsbuildOpts{}
	}

	err := prepareSources(manifest, opts.StoreDir, opts.ExtraEnv, opts.Result, errorWriter)
	if err != nil {
		return nil, fmt.Errorf("Failed to prepare sources: %w", err)
	}

	region, err := awscloud.RegionFromInstanceMetadata()
	if err != nil {
		return nil, fmt.Errorf("Failed to get region from instance metadata: %w", err)
	}

	aws, err := awscloud.NewDefault(region)
	if err != nil {
		return nil, fmt.Errorf("Failed to get default aws client in %s region: %w", region, err)
	}

	si, err := aws.RunSecureInstance(ec2e.iamProfile, ec2e.keyName, ec2e.cloudWatchGroup, ec2e.hostname)
	if err != nil {
		return nil, fmt.Errorf("Unable to start secure instance: %w", err)
	}
	defer func() {
		err := aws.TerminateSecureInstance(si)
		if err != nil {
			logrus.Errorf("Error terminating secure instance: %v", err)
		}
	}()

	executorHost := fmt.Sprintf("http://%s:8001", *si.Instance.PrivateIpAddress)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	if !waitForSI(ctx, executorHost) {
		return nil, fmt.Errorf("Timeout waiting for executor to come online")
	}

	inputArchive, err := writeInputArchive(ec2e.tmpDir, opts.StoreDir, opts.Exports, manifest)
	if err != nil {
		logrus.Errorf("Unable to write input archive: %v", err)
		return nil, err
	}

	osbuildResult, err := handleBuild(inputArchive, executorHost)
	if err != nil {
		logrus.Errorf("Something went wrong handling the executor's build: %v", err)
		return nil, err
	}
	if !osbuildResult.Success {
		return osbuildResult, nil
	}

	outputArchive, err := fetchOutputArchive(ec2e.tmpDir, executorHost)
	if err != nil {
		logrus.Errorf("Unable to fetch executor output: %v", err)
		return nil, err
	}

	err = extractOutputArchive(opts.OutputDir, outputArchive)
	if err != nil {
		logrus.Errorf("Unable to extract executor output: %v", err)
		return nil, err
	}

	return osbuildResult, nil
}

func NewAWSEC2Executor(iamProfile, keyName, cloudWatchGroup, hostname, tmpDir string) Executor {
	return &awsEC2Executor{
		iamProfile,
		keyName,
		cloudWatchGroup,
		hostname,
		tmpDir,
	}
}
