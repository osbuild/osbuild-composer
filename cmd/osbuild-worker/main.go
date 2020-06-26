package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type connectionConfig struct {
	CACertFile     string
	ClientKeyFile  string
	ClientCertFile string
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

	cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
	}, nil
}

func RunTarget(t *target.Target, jobId uuid.UUID, outputDirectory string, uploadFunc func(uuid.UUID, string, io.Reader) error) *worker.TargetError {
	switch options := t.Options.(type) {
	case *target.LocalTargetOptions:
		f, err := os.Open(path.Join(outputDirectory, options.Filename))
		if err != nil {
			// clearly our mistake, let's just panic
			panic(err)
		}

		err = uploadFunc(jobId, options.Filename, f)
		if err != nil {
			return worker.NewTargetError("cannot upload the image to composer: %v", err)
		}

	case *target.AWSTargetOptions:

		a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey)
		if err != nil {
			return worker.NewTargetError("cannot create aws uploader: %v", err)
		}

		if options.Key == "" {
			options.Key = jobId.String()
		}

		_, err = a.Upload(path.Join(outputDirectory, options.Filename), options.Bucket, options.Key)
		if err != nil {
			return worker.NewTargetError("cannot upload the image to aws: %v", err)
		}

		/* TODO: communicate back the AMI */
		_, err = a.Register(t.ImageName, options.Bucket, options.Key)
		if err != nil {
			return worker.NewTargetError("cannot register the image in aws: %v", err)
		}
	case *target.AzureTargetOptions:

		credentials := azure.Credentials{
			StorageAccount:   options.StorageAccount,
			StorageAccessKey: options.StorageAccessKey,
		}
		metadata := azure.ImageMetadata{
			ContainerName: options.Container,
			ImageName:     t.ImageName,
		}

		const azureMaxUploadGoroutines = 4
		err := azure.UploadImage(
			credentials,
			metadata,
			path.Join(outputDirectory, options.Filename),
			azureMaxUploadGoroutines,
		)

		if err != nil {
			return worker.NewTargetError("cannot upload the image to azure: %v", err)
		}
	default:
		return worker.NewTargetError("invalid target type")
	}

	return nil
}

func RunJob(job *worker.Job, store string, uploadFunc func(uuid.UUID, string, io.Reader) error) (result worker.OSBuildJobResult) {
	outputDirectory, err := ioutil.TempDir("/var/tmp", "osbuild-worker-*")
	if err != nil {
		result.GenericError = fmt.Sprintf("error creating temporary output directory: %v", err)
		return
	}
	defer func() {
		err := os.RemoveAll(outputDirectory)
		if err != nil {
			log.Printf("Error removing temporary output directory (%s): %v", outputDirectory, err)
		}
	}()

	osBuildOutput, err := RunOSBuild(job.Manifest, store, outputDirectory, os.Stderr)
	if err != nil {
		if osbuildErr, ok := err.(*OSBuildError); ok {
			result.OSBuildOutput = osbuildErr.Result
			return
		} else {
			result.GenericError = err.Error()
			return
		}
	}

	result.OSBuildOutput = osBuildOutput

	for _, t := range job.Targets {
		targetErr := RunTarget(t, job.Id, outputDirectory, uploadFunc)
		result.Targets = append(result.Targets, worker.TargetResult{Target: *t, Error: targetErr})
	}

	err = os.RemoveAll(outputDirectory)
	if err != nil {
		log.Printf("Error removing osbuild output directory (%s): %v", outputDirectory, err)
	}

	return
}

// Regularly ask osbuild-composer if the compose we're currently working on was
// canceled and exit the process if it was.
// It would be cleaner to kill the osbuild process using (`exec.CommandContext`
// or similar), but osbuild does not currently support this. Exiting here will
// make systemd clean up the whole cgroup and restart this service.
func WatchJob(ctx context.Context, client *worker.Client, job *worker.Job) {
	for {
		select {
		case <-time.After(15 * time.Second):
			if client.JobCanceled(job) {
				log.Println("Job was canceled. Exiting.")
				os.Exit(0)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
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

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		log.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}
	store := path.Join(cacheDirectory, "osbuild-store")

	var client *worker.Client
	if unix {
		client = worker.NewClientUnix(address)
	} else {
		conf, err := createTLSConfig(&connectionConfig{
			CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
			ClientKeyFile:  "/etc/osbuild-composer/worker-key.pem",
			ClientCertFile: "/etc/osbuild-composer/worker-crt.pem",
		})
		if err != nil {
			log.Fatalf("Error creating TLS config: %v", err)
		}

		client = worker.NewClient(address, conf)
	}

	for {
		fmt.Println("Waiting for a new job...")
		job, err := client.AddJob()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Running job %s\n", job.Id)

		ctx, cancel := context.WithCancel(context.Background())
		go WatchJob(ctx, client, job)

		result := RunJob(job, store, client.UploadImage)
		if !result.Successful() {
			resultJson, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				panic(err)
			}
			log.Printf("Job failed, dumping the whole result:\n%s", string(resultJson))
		}

		// signal to WatchJob() that it can stop watching
		cancel()

		err = client.UpdateJob(job, &result)
		if err != nil {
			log.Fatalf("Error reporting job result: %v", err)
		}
	}
}
