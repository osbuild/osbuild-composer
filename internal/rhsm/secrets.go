package rhsm

import (
	"fmt"
	"io/ioutil"
	"os"
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
}

// RHSMSecrets represents a set of CA certificate, client key, and
// client certificate for a specific repository.
type RHSMSecrets struct {
	SSLCACert     string
	SSLClientKey  string
	SSLClientCert string
}

// LoadSystemSubscriptions loads all the available subscriptions.
func LoadSystemSubscriptions() (*Subscriptions, error) {
	// This file has a standard syntax for yum repositories which is
	// documented in `man yum.conf`. The same parsing mechanism could
	// be used for any other repo file in /etc/yum.repos.d/.
	availableSubscriptionsFile := "/etc/yum.repos.d/redhat.repo"
	content, err := ioutil.ReadFile(availableSubscriptionsFile)
	if err != nil {
		if pErr, ok := err.(*os.PathError); ok {
			if pErr.Err.Error() == "no such file or directory" {
				// The system is not subscribed
				return nil, nil
			}
		}
		return nil, fmt.Errorf("Failed to open the file with subscriptions: %w", err)
	}
	subscriptions, err := parseRepoFile(content)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the file with subscriptions: %w", err)
	}

	return &Subscriptions{
		available: subscriptions,
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
		key, err := section.GetKey("baseurl")
		if err != nil {
			continue
		}
		baseurl := key.String()
		key, err = section.GetKey("sslcacert")
		if err != nil {
			continue
		}
		sslcacert := key.String()
		key, err = section.GetKey("sslclientkey")
		if err != nil {
			continue
		}
		sslclientkey := key.String()
		key, err = section.GetKey("sslclientcert")
		if err != nil {
			continue
		}
		sslclientcert := key.String()
		subscriptions = append(subscriptions, subscription{
			id:            id,
			baseurl:       baseurl,
			sslCACert:     sslcacert,
			sslClientKey:  sslclientkey,
			sslClientCert: sslclientcert,
		})
	}

	return subscriptions, nil
}

// GetSecretsForBaseurl queries the Subscriptions structure for a RHSMSecrets of a single repository.
func (s *Subscriptions) GetSecretsForBaseurl(baseurl string, arch, releasever string) (*RHSMSecrets, error) {
	for _, subs := range s.available {
		url := strings.Replace(subs.baseurl, "$basearch", arch, -1)
		url = strings.Replace(url, "$releasever", releasever, -1)
		if url == baseurl {
			return &RHSMSecrets{
				SSLCACert:     subs.sslCACert,
				SSLClientKey:  subs.sslClientKey,
				SSLClientCert: subs.sslClientCert,
			}, nil
		}
	}
	return nil, fmt.Errorf("no such baseurl in the available subscriptions")
}
