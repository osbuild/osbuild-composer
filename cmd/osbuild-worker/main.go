package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
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

type TargetsError struct {
	Errors []error
}

func (e *TargetsError) Error() string {
	errString := fmt.Sprintf("%d target(s) errored:\n", len(e.Errors))

	for _, err := range e.Errors {
		errString += err.Error() + "\n"
	}

	return errString
}

func RunJob(job *worker.Job, uploadFunc func(uuid.UUID, int, io.Reader) error) (*common.ComposeResult, error) {
	tmpStore, err := ioutil.TempDir("/var/tmp", "osbuild-store")
	if err != nil {
		return nil, fmt.Errorf("error setting up osbuild store: %v", err)
	}
	// FIXME: how to handle errors in defer?
	defer os.RemoveAll(tmpStore)

	result, err := RunOSBuild(job.Manifest, tmpStore, os.Stderr)
	if err != nil {
		return nil, err
	}

	var r []error

	for _, t := range job.Targets {
		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:
			f, err := os.Open(path.Join(tmpStore, "refs", result.OutputID, options.Filename))
			if err != nil {
				r = append(r, err)
				continue
			}

			err = uploadFunc(options.ComposeId, options.ImageBuildId, f)
			if err != nil {
				r = append(r, err)
				continue
			}

		case *target.AWSTargetOptions:

			a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey)
			if err != nil {
				r = append(r, err)
				continue
			}

			if options.Key == "" {
				options.Key = job.Id.String()
			}

			_, err = a.Upload(path.Join(tmpStore, "refs", result.OutputID, options.Filename), options.Bucket, options.Key)
			if err != nil {
				r = append(r, err)
				continue
			}

			/* TODO: communicate back the AMI */
			_, err = a.Register(t.ImageName, options.Bucket, options.Key)
			if err != nil {
				r = append(r, err)
				continue
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
				path.Join(tmpStore, "refs", result.OutputID, options.Filename),
				azureMaxUploadGoroutines,
			)

			if err != nil {
				r = append(r, err)
				continue
			}
		default:
			r = append(r, fmt.Errorf("invalid target type"))
		}
	}

	if len(r) > 0 {
		return result, &TargetsError{r}
	}

	return result, nil
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

		var status common.ImageBuildState
		result, err := RunJob(job, client.UploadImage)
		if err != nil {
			log.Printf("  Job failed: %v", err)
			status = common.IBFailed

			// If the error comes from osbuild, retrieve the result
			if osbuildError, ok := err.(*OSBuildError); ok {
				result = osbuildError.Result
			}
		} else {
			status = common.IBFinished
		}

		err = client.UpdateJob(job, status, result)
		if err != nil {
			log.Fatalf("Error reporting job result: %v", err)
		}
	}
}
