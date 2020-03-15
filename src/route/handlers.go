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

const (
	subDelimiter = "-"
	injectedSubs = "injectedSubs"
)

// AdminProxyHandler is Pulsar admin REST api's proxy handler
type AdminProxyHandler struct {
	Destination *url.URL
	Prefix      string
}

// Init initializes database
func Init() {
}

// TokenSubjectHandler issues new token
func TokenSubjectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subject, ok := vars["sub"]
	if !ok {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if util.StrContains(util.SuperRoles, util.AssignString(r.Header.Get("injectedSubs"), "BOGUSROLE")) {
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

// DirectProxyHandler - Pulsar admin REST API reverse proxy
func DirectProxyHandler(w http.ResponseWriter, r *http.Request) {

	log.Printf("%s %v\n", strings.TrimPrefix(r.URL.RequestURI(), "/admin"), util.ProxyURL.Host)

	proxy := httputil.NewSingleHostReverseProxy(util.ProxyURL)
	// Update the headers to allow for SSL redirection
	r.URL.Host = util.ProxyURL.Host
	r.URL.Scheme = util.ProxyURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Proxy", "burnell")
	r.Host = util.ProxyURL.Host

	proxy.ServeHTTP(w, r)

}

// VerifyTenantProxyHandler verifies subject before sending to the proxy URL
func VerifyTenantProxyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if tenantName, ok := vars["tenant"]; ok {
		if VerifySubject(tenantName, r.Header.Get(injectedSubs)) {
			DirectProxyHandler(w, r)
		}
	}
	w.WriteHeader(http.StatusForbidden)
	return
}

// VerifySubject verifies the subject can meet the requirement.
func VerifySubject(requiredSubject, tokenSubjects string) bool {
	for _, v := range strings.Split(tokenSubjects, ",") {
		if util.StrContains(util.SuperRoles, v) {
			return true
		}
		sub := extractSubject(v)
		if sub != "" && requiredSubject == sub {
			return true
		}
	}
	return false
}

func extractSubject(tokenSub string) string {
	// expect - in subject unless it is superuser
	parts := strings.Split(tokenSub, subDelimiter)
	if len(parts) < 2 {
		return ""
	}

	validLength := len(parts) - 1
	return strings.Join(parts[:validLength], subDelimiter)
}
