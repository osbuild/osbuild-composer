package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/upload/vmware"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const configFile = "/etc/osbuild-worker/osbuild-worker.toml"

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

func packageMetadataToSignature(pkg osbuild.RPMPackageMetadata) *string {
	if pkg.SigGPG != "" {
		return &pkg.SigGPG
	} else if pkg.SigPGP != "" {
		return &pkg.SigPGP
	}
	return nil
}

func osbuildStagesToRPMs(stages []osbuild.StageResult) []koji.RPM {
	rpms := make([]koji.RPM, 0)
	for _, stage := range stages {
		switch metadata := stage.Metadata.(type) {
		case *osbuild.RPMStageMetadata:
			for _, pkg := range metadata.Packages {
				rpms = append(rpms, koji.RPM{
					Type:      "rpm",
					Name:      pkg.Name,
					Epoch:     pkg.Epoch,
					Version:   pkg.Version,
					Release:   pkg.Release,
					Arch:      pkg.Arch,
					Sigmd5:    pkg.SigMD5,
					Signature: packageMetadataToSignature(pkg),
				})
			}
		default:
			continue
		}
	}
	return rpms
}

func RunJob(job worker.Job, store string, kojiServers map[string]koji.GSSAPICredentials) (*osbuild.Result, error) {
	outputDirectory, err := ioutil.TempDir("/var/tmp", "osbuild-worker-*")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary output directory: %v", err)
	}
	defer func() {
		err := os.RemoveAll(outputDirectory)
		if err != nil {
			log.Printf("Error removing temporary output directory (%s): %v", outputDirectory, err)
		}
	}()

	args, err := job.OSBuildArgs()
	if err != nil {
		return nil, err
	}

	start_time := time.Now()

	result, err := RunOSBuild(args.Manifest, store, outputDirectory, os.Stderr)
	if err != nil {
		return nil, err
	}

	end_time := time.Now()

	// Don't run uploaders if osbuild failed
	if !result.Success {
		return result, nil
	}

	if job.AcceptsArtifacts() {
		var f *os.File
		imagePath := path.Join(outputDirectory, args.ImageName)
		if args.StreamOptimized {
			f, err = vmware.OpenAsStreamOptimizedVmdk(imagePath)
			if err != nil {
				return nil, err
			}
		} else {
			f, err = os.Open(imagePath)
			if err != nil {
				return nil, err
			}
		}
		err = job.UploadArtifact(args.ImageName, f)
		if err != nil {
			return nil, err
		}
	}

	var r []error

	for _, t := range args.Targets {
		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:
			var f *os.File
			imagePath := path.Join(outputDirectory, options.Filename)
			if options.StreamOptimized {
				f, err = vmware.OpenAsStreamOptimizedVmdk(imagePath)
				if err != nil {
					r = append(r, err)
					continue
				}
			} else {
				f, err = os.Open(imagePath)
				if err != nil {
					r = append(r, err)
					continue
				}
			}

			err = job.UploadArtifact(options.Filename, f)
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

			key := options.Key
			if key == "" {
				key = uuid.New().String()
			}

			_, err = a.Upload(path.Join(outputDirectory, options.Filename), options.Bucket, key)
			if err != nil {
				r = append(r, err)
				continue
			}

			/* TODO: communicate back the AMI */
			_, err = a.Register(t.ImageName, options.Bucket, key)
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
				path.Join(outputDirectory, options.Filename),
				azureMaxUploadGoroutines,
			)

			if err != nil {
				r = append(r, err)
				continue
			}
		case *target.KojiTargetOptions:
			// Koji for some reason needs TLS renegotiation enabled.
			// Clone the default http transport and enable renegotiation.
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{
				Renegotiation: tls.RenegotiateOnceAsClient,
			}

			kojiServer, _ := url.Parse(options.Server)
			creds, exists := kojiServers[kojiServer.Hostname()]
			if !exists {
				r = append(r, fmt.Errorf("Koji server has not been configured: %s", kojiServer.Hostname()))
				continue
			}

			k, err := koji.NewFromGSSAPI(options.Server, &creds, transport)
			if err != nil {
				r = append(r, err)
				continue
			}

			defer func() {
				err := k.Logout()
				if err != nil {
					log.Printf("koji logout failed: %v", err)
				}
			}()

			f, err := os.Open(path.Join(outputDirectory, options.Filename))
			if err != nil {
				r = append(r, err)
				continue
			}

			hash, filesize, err := k.Upload(f, options.UploadDirectory, options.KojiFilename)
			if err != nil {
				r = append(r, err)
				continue
			}

			hostOS, err := distro.GetRedHatRelease()
			if err != nil {
				r = append(r, err)
				continue
			}

			build := koji.ImageBuild{
				BuildID:   options.BuildID,
				TaskID:    options.TaskID,
				Name:      options.Name,
				Version:   options.Version,
				Release:   options.Release,
				StartTime: start_time.Unix(),
				EndTime:   end_time.Unix(),
			}
			buildRoots := []koji.BuildRoot{
				{
					ID: 1,
					Host: koji.Host{
						Os:   hostOS,
						Arch: common.CurrentArch(),
					},
					ContentGenerator: koji.ContentGenerator{
						Name:    "osbuild",
						Version: "0", // TODO: put the correct version here
					},
					Container: koji.Container{
						Type: "none",
						Arch: common.CurrentArch(),
					},
					Tools: []koji.Tool{},
					RPMs:  osbuildStagesToRPMs(result.Build.Stages),
				},
			}
			output := []koji.Image{
				{
					BuildRootID:  1,
					Filename:     options.KojiFilename,
					FileSize:     uint64(filesize),
					Arch:         common.CurrentArch(),
					ChecksumType: "md5",
					MD5:          hash,
					Type:         "image",
					RPMs:         osbuildStagesToRPMs(result.Stages),
					Extra: koji.ImageExtra{
						Info: koji.ImageExtraInfo{
							Arch: "noarch",
						},
					},
				},
			}

			_, err = k.CGImport(build, buildRoots, output, options.UploadDirectory, options.Token)
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

func FailJob(job worker.Job, kojiServers map[string]koji.GSSAPICredentials) {
	args, err := job.OSBuildArgs()
	if err != nil {
		panic(err)
	}

	for _, t := range args.Targets {
		switch options := t.Options.(type) {
		case *target.KojiTargetOptions:
			// Koji for some reason needs TLS renegotiation enabled.
			// Clone the default http transport and enable renegotiation.
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{
				Renegotiation: tls.RenegotiateOnceAsClient,
			}

			kojiServer, _ := url.Parse(options.Server)
			creds, exists := kojiServers[kojiServer.Hostname()]
			if !exists {
				log.Printf("Koji server has not been configured: %s", kojiServer.Hostname())
				return
			}

			k, err := koji.NewFromGSSAPI(options.Server, &creds, transport)
			if err != nil {
				log.Printf("koji login failed: %v", err)
				return
			}

			defer func() {
				err := k.Logout()
				if err != nil {
					log.Printf("koji logout failed: %v", err)
				}
			}()

			err = k.CGFailBuild(int(options.BuildID), options.Token)
			if err != nil {
				log.Printf("CGFailBuild failed: %v", err)
			}
		default:
		}
	}
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
			if err != nil {
				log.Printf("Error fetching job status: %v", err)
				os.Exit(0)
			}
			if canceled {
				log.Println("Job was canceled. Exiting.")
				os.Exit(0)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	var config struct {
		KojiServers map[string]struct {
			Kerberos *struct {
				Principal string `toml:"principal"`
				KeyTab    string `toml:"keytab"`
			} `toml:"kerberos,omitempty"`
		} `toml:"koji"`
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
		log.Println("Composer configuration:")
		encoder := toml.NewEncoder(log.Writer())
		err := encoder.Encode(&config)
		if err != nil {
			log.Fatalf("Could not print config: %v", err)
		}
	} else if !os.IsNotExist(err) {
		log.Fatalf("Could not load config file '%s': %v", configFile, err)
	}

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		log.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}
	store := path.Join(cacheDirectory, "osbuild-store")

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

		client, err = worker.NewClient("https://"+address, conf)
		if err != nil {
			log.Fatalf("Error creating worker client: %v", err)
		}
	}

	for {
		fmt.Println("Waiting for a new job...")
		job, err := client.RequestJob()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Running job %v\n", job.Id())

		ctx, cancel := context.WithCancel(context.Background())
		go WatchJob(ctx, job)

		result, err := RunJob(job, store, kojiServers)
		if err != nil || result.Success == false {
			log.Printf("  Job failed: %v", err)

			// Fail the jobs in any targets that expects it
			FailJob(job, kojiServers)

			// Ensure we always have a non-nil result, composer doesn't like nils.
			// This can happen in cases when OSBuild crashes and doesn't produce
			// a meaningful output. E.g. when the machine runs of disk space.
			if result == nil {
				result = &osbuild.Result{
					Success: false,
				}
			}

			// set the success to false on every error. This is hacky but composer
			// currently relies only on this flag to decide whether a compose was
			// successful. There's no different way how to inform composer that
			// e.g. an upload fail. Therefore, this line reuses the osbuild success
			// flag to indicate all error kinds.
			result.Success = false
		} else {
			log.Printf("  🎉 Job completed successfully: %v", job.Id())
		}

		// signal to WatchJob() that it can stop watching
		cancel()

		err = job.Update(result)
		if err != nil {
			log.Fatalf("Error reporting job result: %v", err)
		}
	}
}
