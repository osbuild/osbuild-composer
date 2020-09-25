package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/kojiapi"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"
	"github.com/osbuild/osbuild-composer/internal/worker"

	"github.com/coreos/go-systemd/activation"
)

const configFile = "/etc/osbuild-composer/osbuild-composer.toml"

type connectionConfig struct {
	// CA used for client certificate validation. If empty, then the CAs
	// trusted by the host system are used.
	CACertFile string

	ServerKeyFile  string
	ServerCertFile string
	AllowedDomains []string
}

func createTLSConfig(c *connectionConfig) (*tls.Config, error) {
	var roots *x509.CertPool

	if c.CACertFile != "" {
		caCertPEM, err := ioutil.ReadFile(c.CACertFile)
		if err != nil {
			return nil, err
		}

		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertPEM)
		if !ok {
			panic("failed to parse root certificate")
		}
	}

	cert, err := tls.LoadX509KeyPair(c.ServerCertFile, c.ServerKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, chain := range verifiedChains {
				for _, domain := range c.AllowedDomains {
					if chain[0].VerifyHostname(domain) == nil {
						return nil
					}
				}
			}

			return errors.New("domain not in allowlist")
		},
	}, nil
}

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	config, err := LoadConfig(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			config = &ComposerConfigFile{}
		} else {
			log.Fatalf("Error loading configuration: %v", err)
		}
	}

	log.Println("Loaded configuration:")
	err = DumpConfig(config, log.Writer())
	if err != nil {
		log.Fatalf("Error printing configuration: %v", err)
	}

	stateDir, ok := os.LookupEnv("STATE_DIRECTORY")
	if !ok {
		log.Fatal("STATE_DIRECTORY is not set. Is the service file missing StateDirectory=?")
	}

	listeners, err := activation.ListenersWithNames()
	if err != nil {
		log.Fatalf("Could not get listening sockets: " + err.Error())
	}

	composerListeners, exists := listeners["osbuild-composer.socket"]
	if !exists {
		log.Fatalf("osbuild-composer.socket doesn't exist")
	}
	if len(composerListeners) != 2 {
		log.Fatalf("Expected two listeners in osbuild-composer.socket, but found %d", len(composerListeners))
	}

	weldrListener := composerListeners[0]
	jobListener := composerListeners[1]

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		log.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}

	rpm := rpmmd.NewRPMMD(path.Join(cacheDirectory, "rpmmd"), "/usr/libexec/osbuild-composer/dnf-json")

	distros, err := distro.NewRegistry(fedora31.New(), fedora32.New(), rhel8.New())
	if err != nil {
		log.Fatalf("Error loading distros: %v", err)
	}

	distribution, beta, err := distros.FromHost()
	if err != nil {
		log.Fatalf("Could not determine distro from host: " + err.Error())
	}

	arch, err := distribution.GetArch(common.CurrentArch())
	if err != nil {
		log.Fatalf("Host distro does not support host architecture: " + err.Error())
	}

	// TODO: refactor to be more generic
	name := distribution.Name()
	if beta {
		name += "-beta"
	}

	repoMap, err := rpmmd.LoadRepositories([]string{"/etc/osbuild-composer", "/usr/share/osbuild-composer"}, name)
	if err != nil {
		log.Fatalf("Could not load repositories for %s: %v", distribution.Name(), err)
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	store := store.New(&stateDir, arch, logger)

	queueDir := path.Join(stateDir, "jobs")
	err = os.Mkdir(queueDir, 0700)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("cannot create queue directory: %v", err)
	}

	// construct job types of the form osbuild:{arch} for all arches
	jobTypes := []string{"osbuild"}
	jobTypesMap := map[string]bool{}
	for _, name := range distros.List() {
		d := distros.GetDistro(name)
		for _, arch := range d.ListArches() {
			jt := "osbuild:" + arch
			if !jobTypesMap[jt] {
				jobTypesMap[jt] = true
				jobTypes = append(jobTypes, jt)
			}
		}
	}

	jobs, err := fsjobqueue.New(queueDir, jobTypes)
	if err != nil {
		log.Fatalf("cannot create jobqueue: %v", err)
	}

	artifactsDir := path.Join(stateDir, "artifacts")
	err = os.Mkdir(artifactsDir, 0755)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("cannot create artifacts directory: %v", err)
	}

	compatOutputDir := path.Join(stateDir, "outputs")

	workers := worker.NewServer(logger, jobs, artifactsDir)
	weldrAPI := weldr.New(rpm, arch, distribution, repoMap[common.CurrentArch()], logger, store, workers, compatOutputDir)

	go func() {
		err := workers.Serve(jobListener)
		common.PanicOnError(err)
	}()

	// Optionally run Koji API
	if kojiListeners, exists := listeners["osbuild-composer-koji.socket"]; exists {
		kojiServers := make(map[string]koji.GSSAPICredentials)
		for server, creds := range config.Koji.Servers {
			if creds.Kerberos == nil {
				// For now we only support Kerberos authentication.
				continue
			}
			kojiServers[server] = koji.GSSAPICredentials{
				Principal: creds.Kerberos.Principal,
				KeyTab:    creds.Kerberos.KeyTab,
			}
		}

		kojiServer := kojiapi.NewServer(logger, workers, rpm, distros, kojiServers)

		tlsConfig, err := createTLSConfig(&connectionConfig{
			CACertFile:     config.Koji.CA,
			ServerKeyFile:  "/etc/osbuild-composer/composer-key.pem",
			ServerCertFile: "/etc/osbuild-composer/composer-crt.pem",
			AllowedDomains: config.Koji.AllowedDomains,
		})
		if err != nil {
			log.Fatalf("TLS configuration cannot be created: " + err.Error())
		}

		if len(kojiListeners) != 1 {
			// Use Fatal to call os.Exit with non-zero return value
			log.Fatal("The osbuild-composer-koji.socket unit is misconfigured. It should contain only one socket.")
		}
		kojiListener := tls.NewListener(kojiListeners[0], tlsConfig)

		go func() {
			err = kojiServer.Serve(kojiListener)

			// If the koji server fails, take down the whole process, not just a single goroutine
			log.Fatal("osbuild-composer-koji.socket failed: ", err)
		}()
	}

	if remoteWorkerListeners, exists := listeners["osbuild-remote-worker.socket"]; exists {
		if len(remoteWorkerListeners) != 1 {
			log.Fatal("The osbuild-remote-worker.socket unit is misconfigured. It should contain only one socket.")
		}

		log.Printf("Starting remote listener\n")

		tlsConfig, err := createTLSConfig(&connectionConfig{
			CACertFile:     config.Worker.CA,
			ServerKeyFile:  "/etc/osbuild-composer/composer-key.pem",
			ServerCertFile: "/etc/osbuild-composer/composer-crt.pem",
			AllowedDomains: config.Worker.AllowedDomains,
		})

		if err != nil {
			log.Fatalf("TLS configuration cannot be created: " + err.Error())
		}

		listener := tls.NewListener(remoteWorkerListeners[0], tlsConfig)
		go func() {
			err := workers.Serve(listener)
			common.PanicOnError(err)
		}()
	}

	err = weldrAPI.Serve(weldrListener)
	common.PanicOnError(err)

}
