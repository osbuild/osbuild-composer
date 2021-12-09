package rhsm

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

type subscription struct {
	id            string
	baseurl       string
	sslCACert     string
	sslClientKey  string
	sslClientCert string
}

// Subscriptions encapsulates all available subscriptions from the
// host system.
type Subscriptions struct {
	available []subscription
	secrets   *RHSMSecrets // secrets are used in there is no matching subscription
}

// RHSMSecrets represents a set of CA certificate, client key, and
// client certificate for a specific repository.
type RHSMSecrets struct {
	SSLCACert     string
	SSLClientKey  string
	SSLClientCert string
}

func getRHSMSecrets() (*RHSMSecrets, error) {
	fmt.Println("[DEBUG] Loading fallback secrets")
	keys, err := filepath.Glob("/etc/pki/entitlement/*-key.pem")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		cert := strings.TrimSuffix(key, "-key.pem") + ".pem"
		if _, err := os.Stat(cert); err == nil {
			fmt.Println("[DEBUG] CA: /etc/rhsm/ca/redhat-uep.pem")
			fmt.Println("[DEBUG] client key: ", key)
			fmt.Println("[DEBUG] client cert: ", cert)
			return &RHSMSecrets{
				SSLCACert:     "/etc/rhsm/ca/redhat-uep.pem",
				SSLClientKey:  key,
				SSLClientCert: cert,
			}, nil
		}
	}
	return nil, fmt.Errorf("no matching key and certificate pair")
}

func getListOfSubscriptions() ([]subscription, error) {
	// This file has a standard syntax for yum repositories which is
	// documented in `man yum.conf`. The same parsing mechanism could
	// be used for any other repo file in /etc/yum.repos.d/.
	availableSubscriptionsFile := "/etc/yum.repos.d/redhat.repo"
	fmt.Println("[DEBUG] reading ", availableSubscriptionsFile, " repository file")
	content, err := ioutil.ReadFile(availableSubscriptionsFile)
	if err != nil {
		if pErr, ok := err.(*os.PathError); ok {
			if pErr.Err.Error() == "no such file or directory" {
				// The system is not subscribed
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to open the file with subscriptions: %w", err)
	}
	subscriptions, err := parseRepoFile(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the file with subscriptions: %w", err)
	}

	return subscriptions, nil
}

// LoadSystemSubscriptions loads all the available subscriptions.
func LoadSystemSubscriptions() (*Subscriptions, error) {
	subscriptions, err1 := getListOfSubscriptions()
	secrets, err2 := getRHSMSecrets()

	if subscriptions == nil && secrets == nil {
		// Neither works, return an error because at least one has to be available
		if err1 != nil {
			return nil, err1
		}
		if err2 != nil {
			return nil, err2
		}
		return nil, fmt.Errorf("failed to load subscriptions")
	}

	return &Subscriptions{
		available: subscriptions,
		secrets:   secrets,
	}, nil
}

func parseRepoFile(content []byte) ([]subscription, error) {
	cfg, err := ini.Load(content)
	if err != nil {
		return nil, err
	}

	subscriptions := make([]subscription, 0)

	for _, section := range cfg.Sections() {
		id := section.Name()
		fmt.Println("[DEBUG] processing section: ", id)
		key, err := section.GetKey("baseurl")
		if err != nil {
			continue
		}
		baseurl := key.String()
		fmt.Println("[DEBUG] baseurl: ", baseurl)
		key, err = section.GetKey("sslcacert")
		if err != nil {
			continue
		}
		sslcacert := key.String()
		fmt.Println("[DEBUG] sslcacert: ", sslcacert)
		key, err = section.GetKey("sslclientkey")
		if err != nil {
			continue
		}
		sslclientkey := key.String()
		fmt.Println("[DEBUG] sslclientkey: ", sslclientkey)
		key, err = section.GetKey("sslclientcert")
		if err != nil {
			continue
		}
		sslclientcert := key.String()
		fmt.Println("[DEBUG] sslclientcert: ", sslclientcert)
		subscriptions = append(subscriptions, subscription{
			id:            id,
			baseurl:       baseurl,
			sslCACert:     sslcacert,
			sslClientKey:  sslclientkey,
			sslClientCert: sslclientcert,
		})
	}

	fmt.Println("[DEBUG] Loaded these subscriptions:")
	for i, s := range subscriptions {
		fmt.Println("[DEBUG] ", i, "# ", s.id)
	}

	return subscriptions, nil
}

// GetSecretsForBaseurl queries the Subscriptions structure for a RHSMSecrets of a single repository.
func (s *Subscriptions) GetSecretsForBaseurl(baseurl string, arch, releasever string) (*RHSMSecrets, error) {
	fmt.Println("[DEBUG] getting secrets for baseurl: ", baseurl, ", arch: ", arch, ", releasever: ", releasever)
	for _, subs := range s.available {
		fmt.Println("[DEBUG] trying baseurl: ", subs.baseurl)
		url := strings.Replace(subs.baseurl, "$basearch", arch, -1)
		url = strings.Replace(url, "$releasever", releasever, -1)
		fmt.Println("[DEBUG] after processing: ", url)
		if url == baseurl {
			fmt.Println("[DEBUG] found a match")
			fmt.Println("[DEBUG] CA: ", subs.sslCACert)
			fmt.Println("[DEBUG] client key: ", subs.sslClientKey)
			fmt.Println("[DEBUG] client cert: ", subs.sslClientCert)
			return &RHSMSecrets{
				SSLCACert:     subs.sslCACert,
				SSLClientKey:  subs.sslClientKey,
				SSLClientCert: subs.sslClientCert,
			}, nil
		}
	}
	fmt.Println("[DEBUG] not found secrets for ", baseurl, " trying fallback")
	// If there is no matching URL, fall back to the global secrets
	if s.secrets != nil {
		fmt.Println("[DEBUG] fallback exists, using it")
		return s.secrets, nil
	}
	fmt.Println("[DEBUG] fallback doesn't exist")
	return nil, fmt.Errorf("no such baseurl in the available subscriptions")
}
