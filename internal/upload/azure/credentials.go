package azure

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Credentials struct {
	clientID     string
	clientSecret string
}

// ParseAzureCredentialsFile parses a credentials file for azure.
// The file is in toml format and contains two keys: client_id and
// client_secret
//
// Example of the file:
// client_id     = "clientIdOfMyApplication"
// client_secret = "ToucanToucan~"
func ParseAzureCredentialsFile(filename string) (*Credentials, error) {
	var creds struct {
		ClientID     string `toml:"client_id"`
		ClientSecret string `toml:"client_secret"`
	}
	_, err := toml.DecodeFile(filename, &creds)
	if err != nil {
		return nil, fmt.Errorf("cannot parse azure credentials: %v", err)
	}

	return &Credentials{
		clientID:     creds.ClientID,
		clientSecret: creds.ClientSecret,
	}, nil
}
