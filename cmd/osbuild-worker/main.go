package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const configFile = "/etc/osbuild-worker/osbuild-worker.toml"
const backoffDuration = time.Second * 10

type connectionConfig struct {
	CACertFile     string
	ClientKeyFile  string
	ClientCertFile string
}

// Represents the implementation of a job type as defined by the worker API.
type JobImplementation interface {
	Run(job worker.Job) error
}

func createTLSConfig(config *connectionConfig) (*tls.Config, error) {
	caCertPEM, err := ioutil.ReadFile(config.CACertFile)
	if err != nil {
		return nil, err
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertPEM)
	if !ok {
		return nil, errors.New("failed to append root certificate")
	}

	var certs []tls.Certificate
	if config.ClientKeyFile != "" && config.ClientCertFile != "" {
		cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}

	return &tls.Config{
		RootCAs:      roots,
		Certificates: certs,
	}, nil
}

// Regularly ask osbuild-composer if the compose we're currently working on was
// canceled and exit the process if it was.
// It would be cleaner to kill the osbuild process using (`exec.CommandContext`
// or similar), but osbuild does not currently support this. Exiting here will
// make systemd clean up the whole cgroup and restart this service.
func WatchJob(ctx context.Context, job worker.Job) {
	for {
		select {
		case <-time.After(15 * time.Second):
			canceled, err := job.Canceled()
			if err == nil && canceled {
				logrus.Info("Job was canceled. Exiting.")
				os.Exit(0)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Requests and runs 1 job of specified type(s)
// Returning an error here will result in the worker backing off for a while and retrying
func RequestAndRunJob(client *worker.Client, acceptedJobTypes []string, jobImpls map[string]JobImplementation) error {
	logrus.Debug("Waiting for a new job...")
	job, err := client.RequestJob(acceptedJobTypes, common.CurrentArch())
	if err == worker.ErrClientRequestJobTimeout {
		logrus.Debugf("Requesting job timed out: %v", err)
		return nil
	}
	if err != nil {
		logrus.Errorf("Requesting job failed: %v", err)
		return err
	}

	impl, exists := jobImpls[job.Type()]
	if !exists {
		logrus.Errorf("Ignoring job with unknown type %s", job.Type())
		return err
	}

	logrus.Infof("Running '%s' job %v\n", job.Type(), job.Id())

	ctx, cancelWatcher := context.WithCancel(context.Background())
	go WatchJob(ctx, job)

	err = impl.Run(job)
	cancelWatcher()
	if err != nil {
		logrus.Warnf("Job %s failed: %v", job.Id(), err)
		// Don't return this error so the worker picks up the next job immediately
		return nil
	}

	logrus.Infof("Job %s finished", job.Id())
	return nil
}

func main() {
	var config struct {
		KojiServers map[string]struct {
			Kerberos *struct {
				Principal string `toml:"principal"`
				KeyTab    string `toml:"keytab"`
			} `toml:"kerberos,omitempty"`
		} `toml:"koji"`
		GCP *struct {
			Credentials string `toml:"credentials"`
		} `toml:"gcp"`
		Azure *struct {
			Credentials string `toml:"credentials"`
		} `toml:"azure"`
		AWS *struct {
			Credentials string `toml:"credentials"`
		} `toml:"aws"`
		Authentication *struct {
			OAuthURL         string `toml:"oauth_url"`
			OfflineTokenPath string `toml:"offline_token"`
		} `toml:"authentication"`
		BasePath string `toml:"base_path"`
	}
	var unix bool
	flag.BoolVar(&unix, "unix", false, "Interpret 'address' as a path to a unix domain socket instead of a network address")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-unix] address\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	flag.Parse()

	address := flag.Arg(0)
	if address == "" {
		flag.Usage()
	}

	_, err := toml.DecodeFile(configFile, &config)
	if err == nil {
		logrus.Info("Composer configuration:")
		encoder := toml.NewEncoder(logrus.StandardLogger().WriterLevel(logrus.InfoLevel))
		err := encoder.Encode(&config)
		if err != nil {
			logrus.Fatalf("Could not print config: %v", err)
		}
	} else if !os.IsNotExist(err) {
		logrus.Fatalf("Could not load config file '%s': %v", configFile, err)
	}

	if config.BasePath == "" {
		config.BasePath = "/api/worker/v1"
	}

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		logrus.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}
	store := path.Join(cacheDirectory, "osbuild-store")
	rpmmd_cache := path.Join(cacheDirectory, "rpmmd")
	output := path.Join(cacheDirectory, "output")
	_ = os.Mkdir(output, os.ModeDir)

	kojiServers := make(map[string]koji.GSSAPICredentials)
	for server, creds := range config.KojiServers {
		if creds.Kerberos == nil {
			// For now we only support Kerberos authentication.
			continue
		}
		kojiServers[server] = koji.GSSAPICredentials{
			Principal: creds.Kerberos.Principal,
			KeyTab:    creds.Kerberos.KeyTab,
		}
	}

	var client *worker.Client
	if unix {
		client = worker.NewClientUnix(address, config.BasePath)
	} else if config.Authentication != nil && config.Authentication.OfflineTokenPath != "" {
		var conf *tls.Config
		conConf := &connectionConfig{
			CACertFile: "/etc/osbuild-composer/ca-crt.pem",
		}
		if _, err = os.Stat(conConf.CACertFile); err == nil {
			conf, err = createTLSConfig(conConf)
			if err != nil {
				logrus.Fatalf("Error creating TLS config: %v", err)
			}
		}

		var token *string
		var oAuthURL *string
		t, err := ioutil.ReadFile(config.Authentication.OfflineTokenPath)
		if err != nil {
			logrus.Fatalf("Could not read offline token: %v", err)
		}
		t2 := strings.TrimSpace(string(t))
		token = &t2

		if config.Authentication.OAuthURL == "" {
			logrus.Fatal("OAuth URL should be specified together with the offline token")
		}
		oAuthURL = &config.Authentication.OAuthURL

		client, err = worker.NewClient(fmt.Sprintf("https://%s", address), conf, token, oAuthURL, config.BasePath)
		if err != nil {
			logrus.Fatalf("Error creating worker client: %v", err)
		}
	} else {
		var conf *tls.Config
		conConf := &connectionConfig{
			CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
			ClientKeyFile:  "/etc/osbuild-composer/worker-key.pem",
			ClientCertFile: "/etc/osbuild-composer/worker-crt.pem",
		}
		if _, err = os.Stat(conConf.CACertFile); err == nil {
			conf, err = createTLSConfig(conConf)
			if err != nil {
				logrus.Fatalf("Error creating TLS config: %v", err)
			}
		}

		client, err = worker.NewClient(fmt.Sprintf("https://%s", address), conf, nil, nil, config.BasePath)
		if err != nil {
			logrus.Fatalf("Error creating worker client: %v", err)
		}
	}

	// Load Azure credentials early. If the credentials file is malformed,
	// we can report the issue early instead of waiting for the first osbuild
	// job with the org.osbuild.azure.image target.
	var azureCredentials *azure.Credentials
	if config.Azure != nil {
		azureCredentials, err = azure.ParseAzureCredentialsFile(config.Azure.Credentials)
		if err != nil {
			logrus.Fatalf("cannot load azure credentials: %v", err)
		}
	}

	// Check if the credentials file was provided in the worker configuration,
	// and load it early to prevent potential failure due to issues with the file.
	// Note that the content validity of the provided file is not checked and
	// can not be reasonable checked with GCP other than by making real API calls.
	var gcpCredentials []byte
	if config.GCP != nil {
		gcpCredentials, err = ioutil.ReadFile(config.GCP.Credentials)
		if err != nil {
			logrus.Fatalf("cannot load GCP credentials: %v", err)
		}
	}

	// If the credentials are not provided in the configuration, then the
	// worker will look in $HOME/.aws/credentials or at the file pointed by
	// the "AWS_SHARED_CREDENTIALS_FILE" variable.
	var awsCredentials = ""
	if config.AWS != nil {
		awsCredentials = config.AWS.Credentials
	}

	// depsolve jobs can be done during other jobs
	depsolveCtx, depsolveCtxCancel := context.WithCancel(context.Background())
	defer depsolveCtxCancel()
	go func() {
		jobImpls := map[string]JobImplementation{
			"depsolve": &DepsolveJobImpl{
				RPMMD: rpmmd.NewRPMMD(rpmmd_cache, "/usr/libexec/osbuild-composer/dnf-json"),
			},
		}
		acceptedJobTypes := []string{}
		for jt := range jobImpls {
			acceptedJobTypes = append(acceptedJobTypes, jt)
		}

		for {
			err := RequestAndRunJob(client, acceptedJobTypes, jobImpls)
			if err != nil {
				logrus.Warn("Received error from RequestAndRunJob, backing off")
				time.Sleep(backoffDuration)
			}

			select {
			case <-depsolveCtx.Done():
				return
			default:
				continue
			}

		}
	}()

	// non-depsolve job
	jobImpls := map[string]JobImplementation{
		"osbuild": &OSBuildJobImpl{
			Store:       store,
			Output:      output,
			KojiServers: kojiServers,
			GCPCreds:    gcpCredentials,
			AzureCreds:  azureCredentials,
			AWSCreds:    awsCredentials,
		},
		"osbuild-koji": &OSBuildKojiJobImpl{
			Store:       store,
			Output:      output,
			KojiServers: kojiServers,
		},
		"koji-init": &KojiInitJobImpl{
			KojiServers: kojiServers,
		},
		"koji-finalize": &KojiFinalizeJobImpl{
			KojiServers: kojiServers,
		},
	}

	acceptedJobTypes := []string{}
	for jt := range jobImpls {
		acceptedJobTypes = append(acceptedJobTypes, jt)
	}

	for {
		err = RequestAndRunJob(client, acceptedJobTypes, jobImpls)
		if err != nil {
			logrus.Warn("Received error from RequestAndRunJob, backing off")
			time.Sleep(backoffDuration)
		}
	}
}
