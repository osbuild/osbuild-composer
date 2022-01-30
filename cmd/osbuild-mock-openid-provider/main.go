package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

// Implements /certs and /token
func main() {
	var addr string
	var rsaPubPem string
	var rsaPem string
	var tlsCert string
	var tlsKey string
	var tokenExpires int
	flag.StringVar(&addr, "a", "localhost:8080", "Address to serve on")
	flag.StringVar(&rsaPubPem, "rsaPubPem", "", "rsa pubkey in pem format (path)")
	flag.StringVar(&rsaPem, "rsaPem", "", "rsa privkey in pem format (path)")
	flag.StringVar(&tlsCert, "cert", "", "tls cert")
	flag.StringVar(&tlsKey, "key", "", "tls key")
	flag.IntVar(&tokenExpires, "expires", 60, "Expiration of the token in seconds (default: 360))")
	flag.Parse()

	if rsaPubPem == "" || rsaPem == "" {
		panic("path to rsa keys needed")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/certs", func(w http.ResponseWriter, r *http.Request) {
		type key struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		}

		rsaPubBytes, err := ioutil.ReadFile(rsaPubPem)
		if err != nil {
			panic(err)
		}
		pubKey, err := jwt.ParseRSAPublicKeyFromPEM(rsaPubBytes)
		if err != nil {
			panic(err)
		}
		k := key{
			Kid: "key-id",
			Kty: "RSA",
			Alg: "RS256",
			N:   strings.TrimRight(base64.URLEncoding.EncodeToString(pubKey.N.Bytes()), "="),
			E:   strings.TrimRight(base64.URLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()), "="),
		}

		type response struct {
			Keys []key `json:"keys"`
		}

		err = json.NewEncoder(w).Encode(response{
			Keys: []key{
				k,
			},
		})
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		type customClaims struct {
			Type      string `json:"typ"`
			ExpiresAt int64  `json:"exp"`
			IssuedAt  int64  `json:"iat"`
			jwt.Claims
		}

		cc := customClaims{
			Type:      "Bearer",
			ExpiresAt: 0,
			IssuedAt:  time.Now().Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, cc)
		token.Header["kid"] = "key-id"

		rsaPrivBytes, err := ioutil.ReadFile(rsaPem)
		if err != nil {
			panic(err)
		}
		privKey, err := jwt.ParseRSAPrivateKeyFromPEM(rsaPrivBytes)
		if err != nil {
			panic(err)
		}
		tokenStr, err := token.SignedString(privKey)
		if err != nil {
			panic(err)
		}

		// See https://datatracker.ietf.org/doc/html/rfc6749
		type response struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`           // required
			ExpiresIn   int    `json:"expires_in,omitempty"` // lifetime in seconds
		}

		err = json.NewEncoder(w).Encode(response{
			AccessToken: tokenStr,
			TokenType:   "Bearer",
			ExpiresIn:   tokenExpires,
		})
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
	})

	if tlsCert != "" && tlsKey != "" {
		log.Fatal(http.ListenAndServeTLS(addr, tlsCert, tlsKey, mux))
	} else {
		log.Fatal(http.ListenAndServe(addr, mux))
	}
}
