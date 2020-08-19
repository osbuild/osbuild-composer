package khttp

import (
	"fmt"
	"github.com/ubccr/kerby"
	"log"
	"net/http"
	"strings"
)

func Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authReq := strings.Split(r.Header.Get(authorizationHeader), " ")
		if len(authReq) != 2 || authReq[0] != negotiateHeader {
			w.Header().Set(wwwAuthenticateHeader, negotiateHeader)
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		ks := new(kerby.KerbServer)
		err := ks.Init("")
		if err != nil {
			log.Printf("KerbServer Init Error: %s", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer ks.Clean()

		err = ks.Step(authReq[1])
		if err != nil {
			log.Printf("KerbServer Step Error: %s", err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set(wwwAuthenticateHeader, fmt.Sprintf("%s %s", negotiateHeader, ks.Response()))
		h.ServeHTTP(w, r)
	})
}
