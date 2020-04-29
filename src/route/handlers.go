package route

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kafkaesque-io/burnell/src/logclient"
	"github.com/kafkaesque-io/burnell/src/metrics"
	"github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/util"

	"github.com/apex/log"
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

// TopicStatsResponse struct
type TopicStatsResponse struct {
	Tenant    string                 `json:"tenant"`
	SessionID string                 `json:"sessionId"`
	Offset    int                    `json:"offset"`
	Total     int                    `json:"total"`
	Data      map[string]interface{} `json:"data"`
}

// AdminProxyHandler is Pulsar admin REST api's proxy handler
type AdminProxyHandler struct {
	Destination *url.URL
	Prefix      string
}

// Init initializes database
func Init() {
	InitCache()
	// CacheTopicStatsWorker()
	// topicStats = make(map[string]map[string]interface{})
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
	log.Infof("hash key is %s\n", key)
	if entry, err := HTTPCache.Get(key); err == nil {
		w.Write(entry)
		w.WriteHeader(http.StatusOK)
		return
	}
	requestURL := util.SingleJoinSlash(util.Config.ProxyURL, r.URL.RequestURI())
	log.Infof("request route %s to proxy %v\n\tdestination url is %s", r.URL.RequestURI(), util.ProxyURL, requestURL)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestURL, nil)
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
	response, err := client.Do(newRequest)
	if err != nil {
		log.Errorf("%v", err)
		util.ResponseErrorJSON(errors.New("proxy failure"), w, http.StatusInternalServerError)
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		util.ResponseErrorJSON(errors.New("failed to read proxy response body"), w, http.StatusInternalServerError)
		return
	}

	err = HTTPCache.Set(key, body)
	if err != nil {
		log.Errorf("Could not write into cache: %v", err)
	}
	log.Debugf("set in cache key is %s", key)

	w.Write(body)
}

// NamespacePolicyProxyHandler - authorizes namespace proxy operation based on tenant plan type
func NamespacePolicyProxyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if tenant, ok := vars["tenant"]; ok {
		if policy.TenantManager.IsFreeStarterPlan(tenant) {
			DirectProxyHandler(w, r)
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
}

// BrokerAggregatorHandler aggregates all broker-stats and reply
func BrokerAggregatorHandler(w http.ResponseWriter, r *http.Request) {
	// RequestURI() should have /admin/v2 to be passed as broker's URL route
	u, _ := url.Parse(r.URL.String())
	params := u.Query()
	offset := queryParamInt(params, "offset", 0)
	limit := queryParamInt(params, "limit", 1) // the limit is per broker
	log.Infof("offset %d limit %d, request subroute %s", offset, limit, r.URL.RequestURI())

	brokerStats, err := policy.AggregateBrokersStats(r.URL.RequestURI(), offset, limit)
	if err != nil {
		http.Error(w, "broker stats error "+err.Error(), http.StatusInternalServerError)
		return
	}

	byte, err := json.Marshal(brokerStats)
	if err != nil {
		http.Error(w, "marshalling broker stats error "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(byte)
	w.WriteHeader(http.StatusOK)
	return
}

// TopicProxyHandler enforces the number of topic based on the plan type
func TopicProxyHandler(w http.ResponseWriter, r *http.Request) {
	limitEnforceProxyHandler(w, r, policy.TenantManager.EvaluateTopicLimit)
}

// NamespaceLimitEnforceProxyHandler enforces the number of namespace limit based on the plan type
func NamespaceLimitEnforceProxyHandler(w http.ResponseWriter, r *http.Request) {
	limitEnforceProxyHandler(w, r, policy.TenantManager.EvaluateNamespaceLimit)
}

func limitEnforceProxyHandler(w http.ResponseWriter, r *http.Request, eval func(tenant string) (bool, error)) {
	if r.Method == http.MethodGet {
		CachedProxyGETHandler(w, r)
		return
	}

	subject := r.Header.Get("injectedSubs")
	if subject == "" {
		http.Error(w, "missing subject", http.StatusUnauthorized)
		return
	}
	_, role := ExtractTenant(subject)
	if util.StrContains(util.SuperRoles, role) {
		DirectProxyHandler(w, r)
		return
	}
	vars := mux.Vars(r)
	if tenant, ok := vars["tenant"]; ok {
		if ok, err := eval(tenant); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		} else if ok {
			DirectProxyHandler(w, r)
		} else {
			http.Error(w, "over the namespace limit", http.StatusForbidden)
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
}

// FunctionLogsHandler responds with the function logs
func FunctionLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	namespace, ok2 := vars["namespace"]
	funcName, ok3 := vars["function"]
	log.WithField("app", "FunctionLogHandler").Debugf("funcation path %s, %s, %s", tenant, namespace, funcName)
	if !(ok && ok2 && ok3) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var reqObj logclient.FunctionLogRequest
	u, _ := url.Parse(r.URL.String())
	params := u.Query()
	reqObj.BackwardPosition = int64(queryParamInt(params, "backwardpos", 0))
	reqObj.ForwardPosition = int64(queryParamInt(params, "forwardpos", 0))
	reqObj.Bytes = int64(queryParamInt(params, "bytes", 0))
	log.WithField("app", "FunctionLogHandler").Infof("function log query params %v", reqObj)
	if reqObj.BackwardPosition > 0 && reqObj.ForwardPosition > 0 {
		http.Error(w, "backwardpos and forwardpos cannot be specified at the same time", http.StatusBadRequest)
	}

	clientRes, err := logclient.GetFunctionLog(tenant+namespace+funcName, reqObj)
	if err != nil {
		if err == logclient.ErrNotFoundFunction {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "log server returned "+err.Error(), http.StatusInternalServerError)
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

	//TODO: disable the feature since the backend database has to populated
	/*if !policy.TenantManager.EvaluateFeatureCode(tenant, policy.BrokerMetrics) {
		http.Error(w, "", http.StatusForbidden)
	}
	*/
	data := metrics.FilterFederatedMetrics(tenant)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}

// TenantTopicStatsHandler returns tenant topic statistics
func TenantTopicStatsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	if !ok {
		http.Error(w, "missing tenant name", http.StatusUnprocessableEntity)
		return
	}
	u, _ := url.Parse(r.URL.String())
	params := u.Query()
	offset := queryParamInt(params, "offset", 0)
	pageSize := queryParamInt(params, "limit", 50)
	log.Debugf("offset %d limit %d", offset, pageSize)

	totalSize, newOffset, topics := policy.PaginateTopicStats(tenant, offset, pageSize)
	if totalSize > 0 {
		data, err := json.Marshal(TopicStatsResponse{
			Tenant:    tenant,
			SessionID: "reserverd for snapshot iteration",
			Offset:    newOffset,
			Total:     totalSize,
			Data:      topics,
		})
		if err != nil {
			http.Error(w, "failed to marshal cached data", http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

func queryParamInt(params url.Values, name string, defaultV int) int {
	if str, ok := params[name]; ok {
		if n, err := strconv.Atoi(str[0]); err == nil {
			return n
		}
	}
	return defaultV
}

// TenantManagementHandler manages tenant CRUD operations.
func TenantManagementHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	if !ok {
		http.Error(w, "missing tenant name", http.StatusUnprocessableEntity)
		return
	}
	var newPlan policy.TenantPlan
	var err error

	switch r.Method {
	case http.MethodGet:
		if newPlan, err = policy.TenantManager.GetTenant(tenant); err != nil {
			util.ResponseErrorJSON(err, w, http.StatusNotFound)
			return
		}

	case http.MethodDelete:
		if newPlan, err = policy.TenantManager.DeleteTenant(tenant); err != nil {
			util.ResponseErrorJSON(err, w, http.StatusInternalServerError)
			return
		}

	case http.MethodPost:
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()

		doc := new(policy.TenantPlan)
		if err := decoder.Decode(doc); err != nil {
			util.ResponseErrorJSON(err, w, http.StatusUnprocessableEntity)
			return
		}

		var statusCode int
		if newPlan, statusCode, err = policy.TenantManager.UpdateTenant(tenant, *doc); err != nil {
			log.Errorf("updateTenant %v", err)
			util.ResponseErrorJSON(err, w, statusCode)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if data, err := json.Marshal(newPlan); err == nil {
		w.Write(data)
	}
	w.WriteHeader(http.StatusOK)
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
	log.Debugf("request route %s proxy URL %v", r.URL.RequestURI(), util.ProxyURL)

	// Update the headers to allow for SSL redirection
	r.URL.Host = util.ProxyURL.Host
	r.URL.Scheme = util.ProxyURL.Scheme
	r.URL.Path = r.URL.RequestURI()
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Proxy", "burnell")
	r.Host = util.ProxyURL.Host
	r.RequestURI = r.URL.RequestURI()
	r.Header["Authorization"] = []string{"Bearer " + util.Config.PulsarToken}
}
