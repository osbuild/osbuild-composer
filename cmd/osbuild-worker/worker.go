package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/cloud/instanceprotector"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type Worker struct {
	config *workerConfig

	jobImplKinds []map[string]worker.JobImplementation

	client *worker.Client
}

func NewWorker(config *workerConfig, unix bool, address, cacheDir string) (*Worker, error) {
	w := Worker{
		config:       config,
		jobImplKinds: make([]map[string]worker.JobImplementation, 0),
	}

	solver := dnfjson.NewBaseSolver(path.Join(cacheDir, "rpmmd"))
	solver.SetDNFJSONPath(w.config.DNFJson)

	store := path.Join(cacheDir, "osbuild-store")
	output := path.Join(cacheDir, "output")
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

	if unix {
		w.client = worker.NewClientUnix(worker.ClientConfig{
			BaseURL:   address,
			BasePath:  config.BasePath,
			Heartbeat: 5 * time.Second,
		})
	} else if config.Authentication != nil {
		var conf *tls.Config
		conConf := &connectionConfig{
			CACertFile: "/etc/osbuild-composer/ca-crt.pem",
		}
		if _, err := os.Stat(conConf.CACertFile); err == nil {
			conf, err = createTLSConfig(conConf)
			if err != nil {
				logrus.Fatalf("Error creating TLS config: %v", err)
			}
		}

		token := ""
		if config.Authentication.OfflineTokenPath != "" {
			t, err := ioutil.ReadFile(config.Authentication.OfflineTokenPath)
			if err != nil {
				logrus.Fatalf("Could not read offline token: %v", err)
			}
			token = strings.TrimSpace(string(t))
		}

		clientSecret := ""
		if config.Authentication.ClientSecretPath != "" {
			cs, err := ioutil.ReadFile(config.Authentication.ClientSecretPath)
			if err != nil {
				logrus.Fatalf("Could not read client secret: %v", err)
			}
			clientSecret = strings.TrimSpace(string(cs))
		}

		proxy := ""
		if config.Composer != nil && config.Composer.Proxy != "" {
			proxy = config.Composer.Proxy
		}

		client, err := worker.NewClient(worker.ClientConfig{
			BaseURL:      fmt.Sprintf("https://%s", address),
			Heartbeat:    15 * time.Second,
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
		w.client = client
	} else {
		var conf *tls.Config
		conConf := &connectionConfig{
			CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
			ClientKeyFile:  "/etc/osbuild-composer/worker-key.pem",
			ClientCertFile: "/etc/osbuild-composer/worker-crt.pem",
		}
		if _, err := os.Stat(conConf.CACertFile); err == nil {
			conf, err = createTLSConfig(conConf)
			if err != nil {
				logrus.Fatalf("Error creating TLS config: %v", err)
			}
		}

		proxy := ""
		if config.Composer != nil && config.Composer.Proxy != "" {
			proxy = config.Composer.Proxy
		}

		client, err := worker.NewClient(worker.ClientConfig{
			BaseURL:   fmt.Sprintf("https://%s", address),
			Heartbeat: 15 * time.Second,
			TlsConfig: conf,
			BasePath:  config.BasePath,
			ProxyURL:  proxy,
		})
		if err != nil {
			logrus.Fatalf("Error creating worker client: %v", err)
		}
		w.client = client
	}

	// Load Azure credentials early. If the credentials file is malformed,
	// we can report the issue early instead of waiting for the first osbuild
	// job with the org.osbuild.azure.image target.
	var azureCredentials *azure.Credentials
	if config.Azure != nil {
		var err error
		azureCredentials, err = azure.ParseAzureCredentialsFile(config.Azure.Credentials)
		if err != nil {
			logrus.Fatalf("cannot load azure credentials: %v", err)
		}
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

	// depsolve jobs can take up to 30 seconds. They are quite CPU intensive
	// but also moderately latency sensitive. A user might be watching the
	// spinner spin. For this reason we don't run them in the same loop as
	// the much slower bulid jobs. We only want to run one depsolve job at
	// a time.
	w.jobImplKinds = append(w.jobImplKinds, map[string]worker.JobImplementation{
		worker.JobTypeDepsolve: &DepsolveJobImpl{
			Solver: solver,
		},
	})

	// build jobs can take up to half an hour. They are both IO and CPU
	// intensive, so we do not want to run multiple at once. They are not
	// particularly CPU intensive.
	w.jobImplKinds = append(w.jobImplKinds, map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &OSBuildJobImpl{
			Store:       store,
			Output:      output,
			KojiServers: kojiServers,
			GCPConfig:   gcpConfig,
			AzureCreds:  azureCredentials,
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
		},
	})

	// the copy and share jobs are not CPU nor IO intensive, but can be
	// fairly long running. We could run several of these concurrently,
	// thouh this is not currently supported.
	w.jobImplKinds = append(w.jobImplKinds, map[string]worker.JobImplementation{
		worker.JobTypeAWSEC2Copy: &AWSEC2CopyJobImpl{
			AWSCreds: awsCredentials,
		},
		worker.JobTypeAWSEC2Share: &AWSEC2ShareJobImpl{
			AWSCreds: awsCredentials,
		},
		worker.JobTypeKojiFinalize: &KojiFinalizeJobImpl{
			KojiServers: kojiServers,
		},
	})

	// the remaining job types are neither CPU nor IO intensive, and are
	// very short running. They could in principle be run concurrently.
	// We run them separately from the other jobs to not add needless
	// latency.
	w.jobImplKinds = append(w.jobImplKinds, map[string]worker.JobImplementation{
		worker.JobTypeKojiInit: &KojiInitJobImpl{
			KojiServers: kojiServers,
		},
		worker.JobTypeContainerResolve: &ContainerResolveJobImpl{
			AuthFilePath: containersAuthFilePath,
		},
		worker.JobTypeOSTreeResolve: &OSTreeResolveJobImpl{},
	})

	return &w, nil
}

func (w *Worker) Start() error {
	// don't schedule new jobs after we receive SIGTERM
	jobCtx, jobCtxCancel := context.WithCancel(context.Background())

	// async protect / unprotect the instances
	ip := instanceprotector.NewInstanceProtector(10*time.Second, time.Minute, 1024, awscloud.NewAWSInstanceProtector())
	go ip.Start(jobCtx)

	for _, jobImpls := range w.jobImplKinds {
		go w.client.Start(jobCtx, backoffDuration, ip, common.CurrentArch(), jobImpls)
	}

	sigint := make(chan os.Signal, 1)

	signal.Notify(sigint, syscall.SIGTERM)
	signal.Notify(sigint, syscall.SIGINT)

	// block until interrupted
	<-sigint

	logrus.Info("Shutting down.")

	jobCtxCancel()

	return nil
}
