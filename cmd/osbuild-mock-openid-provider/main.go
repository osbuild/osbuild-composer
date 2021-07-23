package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

const jwtPublicKeyPEM = `
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAv4FiKP8EBOjIdqkQJAcn
+TjE6w7KPLghlnSMlrGk1PZ1K8CHWYj1xymGYQDhLe0QGtBCZv0+wI1UjAI3xCMU
n+5siufiI6HrtE0ZCUzxEArQMKYxnmZMnl9nuirBevOyFyhx39o2n1iARXTOBrtB
OuWrC1D1EqGyuhS/Tc/Wf/lpKZDCTUwWrfBRxQL1IDKyh5bBWCc9bGoIvPu3Xfdh
FMbK8VRsEw5jq3smKQc7mE4rO3jjJWIyLe09vtVZ3lMP301Ud+cawelTnKhJPMkY
IJl8V39V6f44NjRBvsLoQjPIAVvm4PdvXdzZnqwexr8+vT0Vf3U/klxmbT+2yDgG
uwIDAQAB
-----END PUBLIC KEY-----
`

const jwtPrivateKeyPEM = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAv4FiKP8EBOjIdqkQJAcn+TjE6w7KPLghlnSMlrGk1PZ1K8CH
WYj1xymGYQDhLe0QGtBCZv0+wI1UjAI3xCMUn+5siufiI6HrtE0ZCUzxEArQMKYx
nmZMnl9nuirBevOyFyhx39o2n1iARXTOBrtBOuWrC1D1EqGyuhS/Tc/Wf/lpKZDC
TUwWrfBRxQL1IDKyh5bBWCc9bGoIvPu3XfdhFMbK8VRsEw5jq3smKQc7mE4rO3jj
JWIyLe09vtVZ3lMP301Ud+cawelTnKhJPMkYIJl8V39V6f44NjRBvsLoQjPIAVvm
4PdvXdzZnqwexr8+vT0Vf3U/klxmbT+2yDgGuwIDAQABAoIBAC94LdHFrMRew1oO
fC7CC1mOhdlSODUm20SFLVgpPpd/Y/ntZl9+QJYWp/Whly+gJK7Q0rTer1BheASg
hBw9Kd6e5g7kfbyhZWCy/7K7fMGiPIril0gRSYq0UWznLkCA6bMt1lRLreB/uoP8
+RjYD8o+pdBPSABPTpMrk2QBUcU0qpmT80ngEviKoKGhbmyUDlZvHVOGO91CL8Le
rwvzHixs7OQWTIxY+7ESZoJUN+CH/48WCTaLkBY0Lqt1r3OQ1LlRA2eoMEXb0dEQ
mS17Nq4oDtoP5cZAB9GA+PJlgxpe6K4MkaLbz33HLmUOxtCgqVTs4S6A2grd1dP/
dJv6QvECgYEA7jZdgPso5/NPBkx50kM8K8hz3+iiNaZ7S0sgIFyGSWf3xi+ftrvL
5qEks0riZyaHcRKhIN9wwwGuAME3VOamML4g9521FBeMsXhJ//UE7XJKjWEEqwo3
EVbuWmqYVjs1snz5Mw1WHMa0G+Xbp35n8xgBl1mTkQbeT9wFn6pVoFkCgYEAzc4s
o3wIueJlRGgpUox91PMFwycxtUC9QeVMiKO02llxkIU0jHPef7fvHBrJoYw8sgLO
xY7AdkifyzPqh34n0P3gwcKskRbPzZMpQSHACQ/PvrEpHmogaHOQ0DiApvz8TXIF
015s+4OvwkW922AdBT36ZFRGTujpeQO1mO4XHTMCgYBLKPAbsCNZ/BTlAeA2DWzA
y8Bz12zGzL5+JTf/vfHI23r8Fy6nc12EaTexMmF49lkpvh0EyDtF7BPAvTX+HcA2
BOdV+XaW3k9P94oxrlddrAAF16SnatOxLuKJuLRUEN6CcJgYGY8gCTnuy3mgwWt+
8gYegO7khWxDekJz/ESEEQKBgQCXJtaoF5+9Dia8EBhRVXfRX8+anf2nFm4pqIQG
Ut2wBEMhFoQap7sBaJDHvnDaIkotn1xHwmleNkaOEoosix4pI1zgUd82DGAApxWE
jYoh3agBcNI3UVCOBlqUYvsyKdoP8y+OJuq56uS6NUiUh0mpIPT2nOKqb+uRgoTs
VelJ+wKBgDbkoQHBspO3nYKSCZsyowkcvnmPXVJ7DxNJfHV4a9CcOeki+xKOsnP0
fprqFcln44P+WZLeDePehUDVmkC1aRrtn80hl6BeXYrnW1MgtH1B/mnf49dirbBS
fFTX5kZu9/7hA8ik+zgLzE0i81SliAHYzbuNEMC8U81Yzae+5KwN
-----END RSA PRIVATE KEY-----
`

// Implements /certs and /token
func main() {
	var addr string
	var tlsCert string
	var tlsKey string
	flag.StringVar(&addr, "a", "localhost:8080", "Address to serve on")
	flag.StringVar(&tlsCert, "cert", "", "tls cert")
	flag.StringVar(&tlsKey, "key", "", "tls key")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/certs", func(w http.ResponseWriter, r *http.Request) {
		type key struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		}

		pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(jwtPublicKeyPEM))
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

		privKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(jwtPrivateKeyPEM))
		if err != nil {
			panic(err)
		}
		tokenStr, err := token.SignedString(privKey)
		if err != nil {
			panic(err)
		}

		type response struct {
			AccessToken string `json:"access_token"`
		}

		err = json.NewEncoder(w).Encode(response{
			AccessToken: tokenStr,
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
