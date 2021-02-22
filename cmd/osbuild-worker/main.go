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
	"os"
	"path"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const configFile = "/etc/osbuild-worker/osbuild-worker.toml"

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

	cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
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
		GCP struct {
			Credentials string `toml:"credentials"`
		} `toml:"gcp"`
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

	jobImpls := map[string]JobImplementation{
		"osbuild": &OSBuildJobImpl{
			Store:        store,
			Output:       output,
			KojiServers:  kojiServers,
			GCPCredsPath: config.GCP.Credentials,
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
		fmt.Println("Waiting for a new job...")
		job, err := client.RequestJob(acceptedJobTypes)
		if err != nil {
			log.Fatal(err)
		}

		impl, exists := jobImpls[job.Type()]
		if !exists {
			log.Printf("Ignoring job with unknown type %s", job.Type())
			continue
		}

		fmt.Printf("Running '%s' job %v\n", job.Type(), job.Id())

		ctx, cancelWatcher := context.WithCancel(context.Background())
		go WatchJob(ctx, job)

		err = impl.Run(job)
		cancelWatcher()
		if err != nil {
			log.Printf("Job %s failed: %v", job.Id(), err)
			continue
		}

		log.Printf("Job %s finished", job.Id())
	}
}
