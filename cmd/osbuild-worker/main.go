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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
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

type kojiServer struct {
	creds              koji.GSSAPICredentials
	relaxTimeoutFactor uint
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

// protect an AWS instance from scaling and/or terminating.
func setProtection(protected bool) {
	// create a new session
	awsSession, err := session.NewSession()
	if err != nil {
		logrus.Debugf("Error getting an AWS session, %s", err)
		return
	}

	// get the identity for the instanceID
	identity, err := ec2metadata.New(awsSession).GetInstanceIdentityDocument()
	if err != nil {
		logrus.Debugf("Error getting the identity document, %s", err)
		return
	}

	svc := autoscaling.New(awsSession, aws.NewConfig().WithRegion(identity.Region))

	// get the autoscaling group info for the auto scaling group name
	asInstanceInput := &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []*string{
			aws.String(identity.InstanceID),
		},
	}
	asInstanceOutput, err := svc.DescribeAutoScalingInstances(asInstanceInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			logrus.Warningf("Error getting the Autoscaling instances: %s %s", aerr.Code(), aerr.Error())
		} else {
			logrus.Errorf("Error getting the Autoscaling instances: unknown, %s", err)
		}
		return
	}

	// make the request to protect (or unprotect) the instance
	input := &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: asInstanceOutput.AutoScalingInstances[0].AutoScalingGroupName,
		InstanceIds: []*string{
			aws.String(identity.InstanceID),
		},
		ProtectedFromScaleIn: aws.Bool(protected),
	}
	_, err = svc.SetInstanceProtection(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			logrus.Warningf("Error protecting instance: %s %s", aerr.Code(), aerr.Error())
		} else {
			logrus.Errorf("Error protecting instance: unknown, %s", err)
		}
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

	// Depsolve requests needs reactivity, since setting the protection can take up to 6s to timeout if the worker isn't
	// in an AWS env, disable this setting for them.
	if job.Type() != worker.JobTypeDepsolve {
		setProtection(true)
		defer setProtection(false)
	}

	logrus.Infof("Running job '%s' (%s)\n", job.Id(), job.Type())

	ctx, cancelWatcher := context.WithCancel(context.Background())
	go WatchJob(ctx, job)

	err = impl.Run(job)
	cancelWatcher()
	if err != nil {
		logrus.Warnf("Job '%s' (%s) failed: %v", job.Id(), job.Type(), err)
		// Don't return this error so the worker picks up the next job immediately
		return nil
	}

	logrus.Infof("Job '%s' (%s) finished", job.Id(), job.Type())
	return nil
}

func main() {
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
	encoder := toml.NewEncoder(logrus.StandardLogger().WriterLevel(logrus.InfoLevel))
	err = encoder.Encode(&config)
	if err != nil {
		logrus.Fatalf("Could not print config: %v", err)
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
	var azureCredentials *azure.Credentials
	if config.Azure != nil {
		azureCredentials, err = azure.ParseAzureCredentialsFile(config.Azure.Credentials)
		if err != nil {
			logrus.Fatalf("cannot load azure credentials: %v", err)
		}
	}

	// If the credentials are not provided in the configuration, then the
	// worker will rely on the GCP library to authenticate using default means.
	var gcpCredentials string
	if config.GCP != nil {
		gcpCredentials = config.GCP.Credentials
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

	// depsolve jobs can be done during other jobs
	depsolveCtx, depsolveCtxCancel := context.WithCancel(context.Background())
	solver := dnfjson.NewBaseSolver(rpmmd_cache)
	if config.DNFJson != "" {
		solver.SetDNFJSONPath(config.DNFJson)
	}
	defer depsolveCtxCancel()
	go func() {
		jobImpls := map[string]JobImplementation{
			worker.JobTypeDepsolve: &DepsolveJobImpl{
				Solver: solver,
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
			Store:       store,
			Output:      output,
			KojiServers: kojiServers,
			GCPCreds:    gcpCredentials,
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
		},
		worker.JobTypeOSBuildKoji: &OSBuildKojiJobImpl{
			Store:       store,
			Output:      output,
			KojiServers: kojiServers,
		},
		worker.JobTypeKojiInit: &KojiInitJobImpl{
			KojiServers: kojiServers,
		},
		worker.JobTypeKojiFinalize: &KojiFinalizeJobImpl{
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
