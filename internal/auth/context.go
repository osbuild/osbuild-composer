package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/openshift-online/ocm-sdk-go/authentication"
)

const (
	UsernameKey string = "username"
)

// AuthPayload defines the structure of the JWT payload we expect from
// RHD JWT tokens
type AuthPayload struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Issuer    string `json:"iss"`
	ClientID  string `json:"clientId"`
}

// Get authorization payload api object from context
func GetAuthPayloadFromContext(ctx context.Context) (*AuthPayload, error) {
	// Get user token from request context and validate
	userToken, err := authentication.TokenFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve JWT token from request context: %v", err)
	}

	if userToken == nil {
		return nil, fmt.Errorf("Unable to retrieve JWT token from request context")
	}

	// Username is stored in token claim with key 'sub'
	claims, ok := userToken.Claims.(jwt.MapClaims)
	if !ok {
		err := fmt.Errorf("Unable to parse JWT token claims")
		return nil, err
	}

	// TODO figure out how to unmarshal jwt.mapclaims into the struct to avoid all the
	// type assertions
	//
	//var accountAuth api.AuthPayload
	//err := json.Unmarshal([]byte(claims), &accountAuth)
	//if err != nil {
	//	err := fmt.Errorf("Unable to parse JWT token claims")
	//	return nil, err
	//}

	payload := &AuthPayload{}
	// default to the values we expect from RHSSO
	payload.Username, _ = claims["username"].(string)
	payload.FirstName, _ = claims["first_name"].(string)
	payload.LastName, _ = claims["last_name"].(string)
	payload.Email, _ = claims["email"].(string)
	payload.ClientID, _ = claims["clientId"].(string)

	// Check values, if empty, use alternative claims from RHD
	if payload.Username == "" {
		payload.Username, _ = claims["preferred_username"].(string)
	}

	if payload.FirstName == "" {
		payload.FirstName, _ = claims["given_name"].(string)
	}

	if payload.LastName == "" {
		payload.LastName, _ = claims["family_name"].(string)
	}

	// If given and family names are not present, use the name field
	if payload.FirstName == "" || payload.LastName == "" {
		name, _ := claims["name"].(string)
		names := strings.Split(name, " ")
		if len(names) > 1 {
			payload.FirstName = names[0]
			payload.LastName = names[1]
		} else {
			payload.FirstName = names[0]
		}
	}

	return payload, nil
}
