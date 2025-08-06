package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"time"

	slogger "github.com/osbuild/osbuild-composer/pkg/splunk_logger"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud/azure"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/upload/oci"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const configFile = "/etc/osbuild-worker/osbuild-worker.toml"
const backoffDuration = time.Second * 10

type connectionConfig struct {
	CACertFile     string
	ClientKeyFile  string
	ClientCertFile string
}

type kojiServer struct {
	creds              koji.GSSAPICredentials
	relaxTimeoutFactor time.Duration
}

// Represents the implementation of a job type as defined by the worker API.
type JobImplementation interface {
	Run(job worker.Job) error
}

func createTLSConfig(config *connectionConfig) (*tls.Config, error) {
	caCertPEM, err := os.ReadFile(config.CACertFile)
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
		MinVersion:   tls.VersionTLS12,
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

// Protect an AWS instance from scaling (terminating).
func setProtection(protected bool) {
	// This will fail if the worker isn't running in AWS, so just return with a debug message.
	region, err := awscloud.RegionFromInstanceMetadata()
	if err != nil {
		logrus.Debugf("Error getting the instance region: %v", err)
		return
	}

	aws, err := awscloud.NewDefault(region)
	if err != nil {
		logrus.Infof("Unable to get default aws client: %v", err)
		return
	}

	err = aws.ASGSetProtectHost(protected)
	if err != nil {
		logrus.Infof("Unable to protect host, if the host isn't running as part of an autoscaling group, this can safely be ignored: %v", err)
		return
	}

	if protected {
		logrus.Info("Instance protected")
	} else {
		logrus.Info("Instance protection removed")
	}
}

// Requests and runs 1 job of specified type(s)
// Returning an error here will result in the worker backing off for a while and retrying
func RequestAndRunJob(client *worker.Client, acceptedJobTypes []string, jobImpls map[string]JobImplementation) error {
	logrus.Debug("Waiting for a new job...")
	job, err := client.RequestJob(acceptedJobTypes, arch.Current().String())
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

	// Depsolve requests needs reactivity, since setting the protection can take up to 6s to timeout if the worker isn't
	// in an AWS env, disable this setting for them.
	if job.Type() != worker.JobTypeDepsolve {
		setProtection(true)
		defer setProtection(false)
	}

	logrus.Infof("Running job '%s' (%s)\n", job.Id(), job.Type()) // DO NOT EDIT/REMOVE: used for Splunk dashboard

	ctx, cancelWatcher := context.WithCancel(context.Background())
	go WatchJob(ctx, job)

	err = impl.Run(job)
	cancelWatcher()
	if err != nil {
		logrus.Warnf("Job '%s' (%s) failed: %v", job.Id(), job.Type(), err) // DO NOT EDIT/REMOVE: used for Splunk dashboard
		// Don't return this error so the worker picks up the next job immediately
		return nil
	}

	logrus.Infof("Job '%s' (%s) finished", job.Id(), job.Type()) // DO NOT EDIT/REMOVE: used for Splunk dashboard
	return nil
}

var run = func() {
	var unix bool
	flag.BoolVar(&unix, "unix", false, "Interpret 'address' as a path to a unix domain socket instead of a network address")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-unix] address\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	address := flag.Arg(0)
	if address == "" {
		flag.Usage()
		os.Exit(2)
	}

	config, err := parseConfig(configFile)
	if err != nil {
		logrus.Fatalf("Could not load config file '%s': %v", configFile, err)
	}

	logrus.Info("Composer configuration:")
	configWriter := logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	encoder := toml.NewEncoder(configWriter)
	err = encoder.Encode(&config)
	if err != nil {
		logrus.Fatalf("Could not print config: %v", err)
	}
	configWriter.Close()

	if config.DeploymentChannel != "" {
		logrus.AddHook(&slogger.EnvironmentHook{Channel: config.DeploymentChannel})
	}

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		logrus.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}
	store := path.Join(cacheDirectory, "osbuild-store")
	rpmmd_cache := path.Join(cacheDirectory, "rpmmd")
	output := path.Join(cacheDirectory, "output")
	_ = os.Mkdir(output, os.ModeDir)

	kojiServers := make(map[string]kojiServer)
	for server, kojiConfig := range config.Koji {
		if kojiConfig.Kerberos == nil {
			// For now we only support Kerberos authentication.
			continue
		}
		kojiServers[server] = kojiServer{
			creds: koji.GSSAPICredentials{
				Principal: kojiConfig.Kerberos.Principal,
				KeyTab:    kojiConfig.Kerberos.KeyTab,
			},
			relaxTimeoutFactor: kojiConfig.RelaxTimeoutFactor,
		}
	}

	var client *worker.Client
	if unix {
		client = worker.NewClientUnix(worker.ClientConfig{
			BaseURL:  address,
			BasePath: config.BasePath,
		})
	} else if config.Authentication != nil {
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

		token := ""
		if config.Authentication.OfflineTokenPath != "" {
			t, err := os.ReadFile(config.Authentication.OfflineTokenPath)
			if err != nil {
				logrus.Fatalf("Could not read offline token: %v", err)
			}
			token = strings.TrimSpace(string(t))
		}

		clientSecret := ""
		if config.Authentication.ClientSecretPath != "" {
			cs, err := os.ReadFile(config.Authentication.ClientSecretPath)
			if err != nil {
				logrus.Fatalf("Could not read client secret: %v", err)
			}
			clientSecret = strings.TrimSpace(string(cs))
		}

		proxy := ""
		if config.Composer != nil && config.Composer.Proxy != "" {
			proxy = config.Composer.Proxy
		}

		client, err = worker.NewClient(worker.ClientConfig{
			BaseURL:      fmt.Sprintf("https://%s", address),
			TlsConfig:    conf,
			OfflineToken: token,
			OAuthURL:     config.Authentication.OAuthURL,
			ClientId:     config.Authentication.ClientId,
			ClientSecret: clientSecret,
			BasePath:     config.BasePath,
			ProxyURL:     proxy,
		})
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

		proxy := ""
		if config.Composer != nil && config.Composer.Proxy != "" {
			proxy = config.Composer.Proxy
		}

		client, err = worker.NewClient(worker.ClientConfig{
			BaseURL:   fmt.Sprintf("https://%s", address),
			TlsConfig: conf,
			BasePath:  config.BasePath,
			ProxyURL:  proxy,
		})
		if err != nil {
			logrus.Fatalf("Error creating worker client: %v", err)
		}
	}

	// Load Azure credentials early. If the credentials file is malformed,
	// we can report the issue early instead of waiting for the first osbuild
	// job with the org.osbuild.azure.image target.
	var azureConfig AzureConfiguration
	if config.Azure != nil {
		azureConfig.Creds, err = azure.ParseAzureCredentialsFile(config.Azure.Credentials)
		if err != nil {
			logrus.Fatalf("cannot load azure credentials: %v", err)
		}
		azureConfig.UploadThreads = config.Azure.UploadThreads
	}

	// If the credentials are not provided in the configuration, then the
	// worker will rely on the GCP library to authenticate using default means.
	var gcpConfig GCPConfiguration
	if config.GCP != nil {
		gcpConfig.Creds = config.GCP.Credentials
		gcpConfig.Bucket = config.GCP.Bucket
	}

	// If the credentials are not provided in the configuration, then the
	// worker will look in $HOME/.aws/credentials or at the file pointed by
	// the "AWS_SHARED_CREDENTIALS_FILE" variable.
	var awsCredentials = ""
	var awsBucket = ""
	if config.AWS != nil {
		awsCredentials = config.AWS.Credentials
		awsBucket = config.AWS.Bucket
	}

	var genericS3Credentials = ""
	var genericS3Endpoint = ""
	var genericS3Region = ""
	var genericS3Bucket = ""
	var genericS3CABundle = ""
	var genericS3SkipSSLVerification = false
	if config.GenericS3 != nil {
		genericS3Credentials = config.GenericS3.Credentials
		genericS3Endpoint = config.GenericS3.Endpoint
		genericS3Region = config.GenericS3.Region
		genericS3Bucket = config.GenericS3.Bucket
		genericS3CABundle = config.GenericS3.CABundle
		genericS3SkipSSLVerification = config.GenericS3.SkipSSLVerification
	}

	var containersAuthFilePath string
	var containersDomain = ""
	var containersPathPrefix = ""
	var containersCertPath = ""
	var containersTLSVerify = true
	if config.Containers != nil {
		containersAuthFilePath = config.Containers.AuthFilePath
		containersDomain = config.Containers.Domain
		containersPathPrefix = config.Containers.PathPrefix
		containersCertPath = config.Containers.CertPath
		containersTLSVerify = config.Containers.TLSVerify
	}

	var ociConfig OCIConfiguration
	if config.OCI != nil {
		var creds struct {
			User        string `toml:"user"`
			Tenancy     string `toml:"tenancy"`
			Region      string `toml:"region"`
			Fingerprint string `toml:"fingerprint"`
			PrivateKey  string `toml:"private_key"`
			Bucket      string `toml:"bucket"`
			Namespace   string `toml:"namespace"`
			Compartment string `toml:"compartment"`
		}
		_, err := toml.DecodeFile(config.OCI.Credentials, &creds)
		if err != nil {
			logrus.Fatalf("cannot load oci credentials: %v", err)
		}
		ociConfig = OCIConfiguration{
			ClientParams: &oci.ClientParams{
				User:        creds.User,
				Region:      creds.Region,
				Tenancy:     creds.Tenancy,
				PrivateKey:  creds.PrivateKey,
				Fingerprint: creds.Fingerprint,
			},
			Bucket:      creds.Bucket,
			Namespace:   creds.Namespace,
			Compartment: creds.Compartment,
		}
	}

	var pulpCredsFilePath = ""
	var pulpAddress = ""
	if config.Pulp != nil {
		pulpCredsFilePath = config.Pulp.Credentials
		pulpAddress = config.Pulp.ServerURL
	}

	var repositoryMTLSConfig *RepositoryMTLSConfig
	if config.RepositoryMTLSConfig != nil {
		baseURL, err := url.Parse(config.RepositoryMTLSConfig.BaseURL)
		if err != nil {
			logrus.Fatalf("Repository MTL baseurl not valid: %v", err)
		}

		var proxyURL *url.URL
		if config.RepositoryMTLSConfig.Proxy != "" {
			proxyURL, err = url.Parse(config.RepositoryMTLSConfig.Proxy)
			if err != nil {
				logrus.Fatalf("Repository Proxy url not valid: %v", err)
			}
		}

		repositoryMTLSConfig = &RepositoryMTLSConfig{
			BaseURL:        baseURL,
			CA:             config.RepositoryMTLSConfig.CA,
			MTLSClientKey:  config.RepositoryMTLSConfig.MTLSClientKey,
			MTLSClientCert: config.RepositoryMTLSConfig.MTLSClientCert,
			Proxy:          proxyURL,
		}
	}

	// depsolve and search jobs can be done during other jobs
	depsolveCtx, depsolveCtxCancel := context.WithCancel(context.Background())
	solver := dnfjson.NewBaseSolver(rpmmd_cache)
	if config.DNFJson != "" {
		solver.SetDNFJSONPath(config.DNFJson)
	}
	defer depsolveCtxCancel()
	go func() {
		jobImpls := map[string]JobImplementation{
			worker.JobTypeDepsolve: &DepsolveJobImpl{
				Solver:               solver,
				RepositoryMTLSConfig: repositoryMTLSConfig,
			},
			worker.JobTypeSearchPackages: &SearchPackagesJobImpl{
				Solver:               solver,
				RepositoryMTLSConfig: repositoryMTLSConfig,
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
		worker.JobTypeOSBuild: &OSBuildJobImpl{
			Store:  store,
			Output: output,
			OSBuildExecutor: ExecutorConfiguration{
				Type:            config.OSBuildExecutor.Type,
				IAMProfile:      config.OSBuildExecutor.IAMProfile,
				KeyName:         config.OSBuildExecutor.KeyName,
				CloudWatchGroup: config.OSBuildExecutor.CloudWatchGroup,
			},
			KojiServers: kojiServers,
			GCPConfig:   gcpConfig,
			AzureConfig: azureConfig,
			OCIConfig:   ociConfig,
			AWSCreds:    awsCredentials,
			AWSBucket:   awsBucket,
			S3Config: S3Configuration{
				Creds:               genericS3Credentials,
				Endpoint:            genericS3Endpoint,
				Region:              genericS3Region,
				Bucket:              genericS3Bucket,
				CABundle:            genericS3CABundle,
				SkipSSLVerification: genericS3SkipSSLVerification,
			},
			ContainersConfig: ContainersConfiguration{
				AuthFilePath: containersAuthFilePath,
				Domain:       containersDomain,
				PathPrefix:   containersPathPrefix,
				CertPath:     containersCertPath,
				TLSVerify:    &containersTLSVerify,
			},
			PulpConfig: PulpConfiguration{
				CredsFilePath: pulpCredsFilePath,
				ServerAddress: pulpAddress,
			},
			RepositoryMTLSConfig: repositoryMTLSConfig,
		},
		worker.JobTypeKojiInit: &KojiInitJobImpl{
			KojiServers: kojiServers,
		},
		worker.JobTypeKojiFinalize: &KojiFinalizeJobImpl{
			KojiServers: kojiServers,
		},
		worker.JobTypeContainerResolve: &ContainerResolveJobImpl{
			AuthFilePath: containersAuthFilePath,
		},
		worker.JobTypeOSTreeResolve: &OSTreeResolveJobImpl{
			RepositoryMTLSConfig: repositoryMTLSConfig,
		},
		worker.JobTypeFileResolve: &FileResolveJobImpl{},
		worker.JobTypeAWSEC2Copy: &AWSEC2CopyJobImpl{
			AWSCreds: awsCredentials,
		},
		worker.JobTypeAWSEC2Share: &AWSEC2ShareJobImpl{
			AWSCreds: awsCredentials,
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

func main() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Fatalf("worker crashed: %s\n%s", r, debug.Stack())
		}
	}()

	run()
}
