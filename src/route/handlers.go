package route

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kafkaesque-io/burnell/src/logclient"
	"github.com/kafkaesque-io/burnell/src/metrics"
	"github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/util"
)

const (
	subDelimiter = "-"
	injectedSubs = "injectedSubs"
)

// TokenServerResponse is the json object for token server response
type TokenServerResponse struct {
	Subject string `json:"subject"`
	Token   string `json:"token"`
}

// AdminProxyHandler is Pulsar admin REST api's proxy handler
type AdminProxyHandler struct {
	Destination *url.URL
	Prefix      string
}

// Init initializes database
func Init() {
	InitCache()
}

// TokenSubjectHandler issues new token
func TokenSubjectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subject, ok := vars["sub"]
	if !ok {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	tokenString, err := util.JWTAuth.GenerateToken(subject)
	if err != nil {
		util.ResponseErrorJSON(errors.New("failed to generate token"), w, http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		respJSON, err := json.Marshal(&TokenServerResponse{
			Subject: subject,
			Token:   tokenString,
		})
		if err != nil {
			util.ResponseErrorJSON(errors.New("failed to marshal token response json object"), w, http.StatusInternalServerError)
			return
		}
		w.Write(respJSON)
		w.WriteHeader(http.StatusOK)
		return
	}
	return
}

// StatusPage replies with basic status code
func StatusPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	return
}

// DirectProxyHandler - Pulsar admin REST API reverse proxy with caching
func DirectProxyHandler(w http.ResponseWriter, r *http.Request) {

	proxy := httputil.NewSingleHostReverseProxy(util.ProxyURL)
	// Update r *http.Request based on proxy
	updateProxyRequest(r)

	proxy.ServeHTTP(w, r)

}

// VerifyTenantCachedProxyHandler verifies subject before sending to the proxy URL
func VerifyTenantCachedProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := VerifyTenant(r); ok {
		CachedProxyHandler(w, r)
	}
	w.WriteHeader(http.StatusForbidden)
	return
}

// VerifyTenantNamespaceCachedProxyHandler verifies subject before sending to the proxy URL
func VerifyTenantNamespaceCachedProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, sub, ok := VerifyTenant(r); ok {
		if policy.EvalNamespaceAdminAPI(r, sub) {
			CachedProxyHandler(w, r)
		}
	}
	w.WriteHeader(http.StatusForbidden)
	return
}

// VerifyTenantTopicCachedProxyHandler verifies subject before sending to the proxy URL
func VerifyTenantTopicCachedProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, sub, ok := VerifyTenant(r); ok {
		if policy.EvalNamespaceAdminAPI(r, sub) {
			CachedProxyHandler(w, r)
		}
	}
	w.WriteHeader(http.StatusForbidden)
	return
}

// CachedProxyHandler is
func CachedProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		CachedProxyGETHandler(w, r)
		return
	}
	DirectProxyHandler(w, r)
}

// CachedProxyGETHandler is a http proxy handler with caching capability for GET method only.
func CachedProxyGETHandler(w http.ResponseWriter, r *http.Request) {
	key := HashKey(r.URL.Path + r.Header["Authorization"][0])
	log.Printf("hash key is %s\n", key)
	if entry, err := HttpCache.Get(key); err == nil {
		log.Println("found in cache")
		w.Write(entry)
		w.WriteHeader(http.StatusOK)
		return
	}
	requestRoute := strings.TrimPrefix(r.URL.RequestURI(), util.AssignString(util.Config.AdminRestPrefix, "/admin/"))
	log.Printf(" proxy %s %v request route is %s\n", r.URL.RequestURI(), util.ProxyURL, requestRoute)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, util.Config.ProxyURL+requestRoute, nil)
	if err != nil {
		util.ResponseErrorJSON(errors.New("failed to set proxy request"), w, http.StatusInternalServerError)
		return
	}
	newRequest.Header.Add("X-Forwarded-Host", r.Header.Get("Host"))
	newRequest.Header.Add("X-Proxy", "burnell")
	//r.Host = util.ProxyURL.Host
	//r.RequestURI = util.ProxyURL.RequestURI() + requestRoute
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	log.Printf("r requestURI %s\nproxy r:: %v\n", newRequest.RequestURI, newRequest)
	response, err := client.Do(newRequest)
	if err != nil {
		log.Println(err)
		util.ResponseErrorJSON(errors.New("proxy failure"), w, http.StatusInternalServerError)
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		util.ResponseErrorJSON(errors.New("failed to read proxy response body"), w, http.StatusInternalServerError)
		return
	}

	err = HttpCache.Set(key, body)
	if err != nil {
		log.Printf("Could not write into cache: %s", err)
	}
	log.Printf("set in cache key is %s\n", key)

	w.Write(body)
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

// FunctionLogsHandler responds with the function logs
func FunctionLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	namespace, ok2 := vars["namespace"]
	funcName, ok3 := vars["function"]
	// fmt.Printf("%s, %s %s\n", tenant, namespace, funcName)
	if !(ok && ok2 && ok3) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var reqObj logclient.FunctionLogRequest
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	decoder.Decode(&reqObj)

	clientRes, err := logclient.GetFunctionLog(tenant+namespace+funcName, reqObj)
	if err != nil {
		if err == logclient.ErrNotFoundFunction {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// fmt.Printf("pos %d, %d\n", clientRes.BackwardPosition, clientRes.ForwardPosition)
	jsonResponse, err := json.Marshal(clientRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
	return
}

// PulsarFederatedPrometheusHandler exposes pulsar federated prometheus metrics
func PulsarFederatedPrometheusHandler(w http.ResponseWriter, r *http.Request) {
	subject := r.Header.Get("injectedSubs")
	if subject == "" {
		http.Error(w, "missing subject", http.StatusUnauthorized)
		return
	}
	_, tenant := ExtractTenant(subject)
	// fmt.Printf("subject for federated prom %s tenant %s\n", subject, tenant)
	if util.StrContains(util.SuperRoles, tenant) {
		w.Write([]byte(metrics.AllNamespaceMetrics()))
		w.WriteHeader(http.StatusOK)
		return
	}
	data := metrics.FilterFederatedMetrics(tenant)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}

// VerifyTenant verifies tenant and returns tenant name and weather verification has passed
func VerifyTenant(r *http.Request) (string, string, bool) {
	vars := mux.Vars(r)
	sub := r.Header.Get(injectedSubs)
	if tenantName, ok := vars["tenant"]; ok {
		if VerifySubject(tenantName, sub) {
			return tenantName, sub, true
		}
	}
	return "", sub, false
}

// VerifySubject verifies the subject can meet the requirement.
func VerifySubject(requiredSubject, tokenSubjects string) bool {
	for _, v := range strings.Split(tokenSubjects, ",") {
		if util.StrContains(util.SuperRoles, v) {
			return true
		}
		subCase1, subCase2 := ExtractTenant(v)
		return requiredSubject == subCase1 || requiredSubject == subCase2
	}
	return false
}

// ExtractTenant attempts to extract tenant based on delimiter `-` and `-client-`
// so that it will covercases such as 1. chris-kafkaesque-io-12345qbc
// 2. chris-kafkaesque-io-client-12345qbc
// 3. chris-kafkaesque-io
// 4. chris-kafkaesque-io-client-client-12345qbc
func ExtractTenant(tokenSub string) (string, string) {
	var case1 string
	// expect `-` in subject unless it is superuser, or admin
	// so return them as is
	parts := strings.Split(tokenSub, subDelimiter)
	if len(parts) < 2 {
		return tokenSub, tokenSub
	}

	// cases to cover with only `-` as delimiter
	validLength := len(parts) - 1
	case1 = strings.Join(parts[:validLength], subDelimiter)

	if parts[validLength-1] == "client" {
		return case1, strings.Join(parts[:(validLength-1)], subDelimiter)
	}
	return case1, case1
}

func updateProxyRequest(r *http.Request) {
	requestRoute := strings.TrimPrefix(r.URL.RequestURI(), util.AssignString(util.Config.AdminRestPrefix, "/admin/"))
	log.Printf("direct proxy %s %v request route is %s\n", r.URL.RequestURI(), util.ProxyURL, requestRoute)

	// Update the headers to allow for SSL redirection
	r.URL.Host = util.ProxyURL.Host
	r.URL.Scheme = util.ProxyURL.Scheme
	r.URL.Path = requestRoute
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Proxy", "burnell")
	r.Host = util.ProxyURL.Host
	r.RequestURI = util.ProxyURL.RequestURI() + requestRoute
	r.Header["Authorization"] = []string{"Bearer " + util.Config.PulsarToken}
	//log.Printf("r requestURI %s\nproxy r:: %v\n", r.RequestURI, r)
}
