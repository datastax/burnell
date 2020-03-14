package route

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kafkaesque-io/burnell/src/util"
)

var superRoles []string

var proxyURL *url.URL

// Init initializes database
func Init() {
	if uri, err := url.ParseRequestURI(util.Config.ProxyURL); err != nil {
		log.Fatal(err)
	} else {
		proxyURL = uri
	}
}

// TokenSubjectHandler issues new token
func TokenSubjectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subject, ok := vars["sub"]
	if !ok {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if util.StrContains(superRoles, util.AssignString(r.Header.Get("injectedSubs"), "BOGUSROLE")) {
		tokenString, err := util.JWTAuth.GenerateToken(subject)
		if err != nil {
			util.ResponseErrorJSON(errors.New("failed to generate token"), w, http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(tokenString))
		}
		return
	}
	util.ResponseErrorJSON(errors.New("incorrect subject"), w, http.StatusUnauthorized)
	return
}

// StatusPage replies with basic status code
func StatusPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	return
}

// AdminProxyHandler - Pulsar admin REST API reverse proxy
func AdminProxyHandler(w http.ResponseWriter, r *http.Request) {

	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	// Update the headers to allow for SSL redirection
	r.URL.Host = proxyURL.Host
	r.URL.Scheme = proxyURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Proxy", "burnell")
	r.Host = proxyURL.Host

	proxy.ServeHTTP(w, r)
}

// VerifySubject verifies the subject can meet the requirement.
func VerifySubject(topicFN, tokenSub string) bool {
	parts := strings.Split(topicFN, "/")
	if len(parts) < 3 {
		return false
	}
	tenant := parts[2]
	if len(tenant) < 1 {
		log.Printf(" auth verify tenant %s token sub %s", tenant, tokenSub)
		return false
	}
	subjects := append(superRoles, tenant)

	return util.StrContains(subjects, tokenSub)
}
