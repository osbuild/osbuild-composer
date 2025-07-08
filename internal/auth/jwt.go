package auth

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v4"
	"github.com/openshift-online/ocm-sdk-go/authentication"
)

var ErrNoJWT = errors.New("request doesn't contain JWT")
var ErrNoKey = errors.New("cannot find key in jwt claims")

// GetFromClaims returns a value of JWT claim with the specified key
//
// Caller can specify multiple keys. The value of first one that exists and is
// non-empty is returned.
//
// If no claim is found, NoKeyError is returned
func GetFromClaims(ctx context.Context, keys []string) (string, error) {
	token, err := authentication.TokenFromContext(ctx)
	if err != nil {
		return "", err
	} else if token == nil {
		return "", ErrNoJWT
	}

	claims := token.Claims.(jwt.MapClaims)
	for _, f := range keys {
		value, exists := claims[f]
		valueStr, isString := value.(string)
		if exists && isString && valueStr != "" {
			return valueStr, nil
		}

	}

	return "", ErrNoKey
}
