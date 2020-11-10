package route

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kafkaesque-io/burnell/src/logclient"
	"github.com/kafkaesque-io/burnell/src/metrics"
	"github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/util"
	"github.com/kafkaesque-io/pulsar-beam/src/model"
	"github.com/kafkaesque-io/pulsar-beam/src/route"

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
	if !util.IsPulsarJWTEnabled() {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
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
		respJSON, err := json.Marshal(&TokenServerResponse{
			Subject: subject,
			Token:   tokenString,
		})
		if err != nil {
			util.ResponseErrorJSON(errors.New("failed to marshal token response json object"), w, http.StatusInternalServerError)
			return
		}
		w.Write(respJSON) // implicitly http.StatusOK
		return
	}
	return
}

// StatusPage replies with basic status code
func StatusPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	return
}

// DirectBrokerProxyHandler - Pulsar broker admin REST API
func DirectBrokerProxyHandler(w http.ResponseWriter, r *http.Request) {
	requestURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, r.URL.RequestURI())
	httpProxy(requestURL, w, r)
}

// DirectFunctionProxyHandler - Pulsar function admin REST API
func DirectFunctionProxyHandler(w http.ResponseWriter, r *http.Request) {
	// w.Header().Del("Content-Type") // remove middle set content-type because the proxy will set too
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		subject := r.Header.Get("injectedSubs")
		if subject == "" {
			http.Error(w, "missing subject", http.StatusUnauthorized)
			return
		}
		_, role := ExtractTenant(subject)
		isSuperUser := util.StrContains(util.SuperRoles, role)
		vars := mux.Vars(r)
		if tenant, ok := vars["tenant"]; ok {
			limit := policy.TenantManager.GetFunctionsLimit(tenant)
			log.Infof("tenant %s with function limit %d, actual counts %d, is superuser %v", tenant, logclient.TenantFunctionCount(tenant), limit, isSuperUser)
			if logclient.TenantFunctionCount(tenant) >= limit && !isSuperUser {
				http.Error(w, "over the number of function limit under the current plan, please upgrade your plan", http.StatusPaymentRequired)
				return
			}
		}
	}

	requestURL := util.SingleJoinSlash(util.Config.FunctionProxyURL, r.URL.RequestURI())
	httpProxy(requestURL, w, r)
}

// RestrictedTenantsProxyHandler filters tenants based on token subject
func RestrictedTenantsProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		util.ResponseErrorJSON(errors.New("only GET method allowed"), w, http.StatusMethodNotAllowed)
		return
	}

	data, statusCode, err := cachedGetProxy(r)
	if err != nil {
		util.ResponseErrorJSON(err, w, statusCode)
		return
	}

	subject := r.Header.Get("injectedSubs")
	if subject == "" {
		http.Error(w, "missing subject", http.StatusUnauthorized)
		return
	}
	_, role := ExtractTenant(subject)
	if util.StrContains(util.SuperRoles, role) {
		w.Write(data)
		return
	}
	log.Infof("subject role is %s", role)

	// if superrole just return
	var tenants []string
	err = json.Unmarshal(data, &tenants)
	if err != nil {
		log.Errorf("unmarshal error %v", err)
		util.ResponseErrorJSON(err, w, http.StatusInternalServerError)
		return
	}

	for _, v := range tenants {
		if strings.HasPrefix(subject, v) {
			data, err := json.Marshal([]string{v})
			if err != nil {
				util.ResponseErrorJSON(errors.New("failed to build a list of tenants"), w, http.StatusInternalServerError)
				return
			}
			w.Write(data)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	return
}

// CachedProxyHandler is
func CachedProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		CachedProxyGETHandler(w, r)
		return
	}
	DirectBrokerProxyHandler(w, r)
}

// CachedProxyGETHandler is a http proxy handler with caching capability for GET method only.
func CachedProxyGETHandler(w http.ResponseWriter, r *http.Request) {
	data, statusCode, err := cachedGetProxy(r)
	if err == nil {
		log.Infof("CachedProxyGETHandler return status %d", statusCode)
		w.WriteHeader(statusCode)
		w.Write(data)
		return
	}
	util.ResponseErrorJSON(err, w, statusCode)
	return
}

func cachedGetProxy(r *http.Request) ([]byte, int, error) {
	//key := HashKey(r.URL.Path + r.Header["Authorization"][0])
	//log.Infof("hash key is %s\n", key)
	//if entry, err := HTTPCache.Get(key); err == nil {
	//	return entry, http.StatusOK, nil
	//}
	requestURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, r.URL.RequestURI())
	log.Infof("request route %s to proxy %v\n\tdestination url is %s", r.URL.RequestURI(), util.BrokerProxyURL, requestURL)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		// util.ResponseErrorJSON(errors.New("failed to set proxy request"), w, http.StatusInternalServerError)
		return nil, http.StatusInternalServerError, err
	}
	newRequest.Header = r.Header
	newRequest.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	newRequest.Header.Set("X-Proxy", "burnell")
	//r.Host = util.ProxyURL.Host
	//r.RequestURI = util.ProxyURL.RequestURI() + requestRoute
	newRequest.Header.Set("Authorization", "Bearer "+util.Config.PulsarToken)

	client := &http.Client{
		CheckRedirect: util.PreserveHeaderForRedirect,
	}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		log.Errorf("%v", err)
		return nil, http.StatusInternalServerError, errors.New("proxy failure")
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("failed to read proxy response body")
	}

	/*err = HTTPCache.Set(key, body)
	if err != nil {
		log.Errorf("Could not write into cache: %v", err)
	}
	log.Debugf("set in cache key is %s", key)
	*/

	return body, response.StatusCode, nil
}

func httpProxy(requestURL string, w http.ResponseWriter, r *http.Request) {
	log.Infof("request route %s to proxy %v\n\tmethod %v destination url is %s", r.URL.RequestURI(), util.BrokerProxyURL, r.Method, requestURL)

	body, err := ioutil.ReadAll(r.Body)
	if body != nil {
		defer r.Body.Close()
	}
	if err != nil {
		log.Infof("%s Error reading body: %v", requestURL, err)
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(r.Method, requestURL, bytes.NewBuffer(body))
	if err != nil {
		util.ResponseErrorJSON(errors.New("failed to set proxy request"), w, http.StatusInternalServerError)
		return
	}
	if newRequest.Header == nil {
		newRequest.Header = make(http.Header)
	}
	newRequest.Header = r.Header
	newRequest.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	newRequest.Header.Set("X-Proxy", "burnell")
	newRequest.Header.Set("Authorization", "Bearer "+util.Config.PulsarToken)

	client := &http.Client{
		CheckRedirect: util.PreserveHeaderForRedirect,
	}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		log.Errorf("%v", err)
		util.ResponseErrorJSON(errors.New("proxy failure"), w, http.StatusInternalServerError)
		return
	}

	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		util.ResponseErrorJSON(errors.New("failed to read proxy response body"), w, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(response.StatusCode)
	w.Write(body)
	return
}

func getTenantNameList() ([]string, error) {
	requestURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, "admin/v2/tenants")
	newRequest, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		// util.ResponseErrorJSON(errors.New("failed to set proxy request"), w, http.StatusInternalServerError)
		return nil, err
	}
	newRequest.Header.Add("X-Proxy", "burnell")
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)

	client := &http.Client{
		CheckRedirect: util.PreserveHeaderForRedirect,
	}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		log.Errorf("%v", err)
		return nil, errors.New("proxy failure")
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("failed to read proxy response body")
	}

	tenants := []string{}
	if err = json.Unmarshal(body, &tenants); err != nil {
		return nil, err
	}

	return tenants, nil
}

// NamespacePolicyProxyHandler - authorizes namespace proxy operation based on tenant plan type
func NamespacePolicyProxyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if tenant, ok := vars["tenant"]; ok {
		if policy.TenantManager.IsFreeStarterPlan(tenant) {
			DirectBrokerProxyHandler(w, r)
		}
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// BrokerAggregatorHandler aggregates all broker-stats and reply
func BrokerAggregatorHandler(w http.ResponseWriter, r *http.Request) {
	// RequestURI() should have /admin/v2 to be passed as broker's URL route
	u, _ := url.Parse(r.URL.String())
	params := u.Query()
	offset := queryParamInt(params, "offset", 0)
	limit := queryParamInt(params, "limit", 0) // the limit is per broker
	log.Infof("offset %d limit %d, request subroute %s", offset, limit, r.URL.RequestURI())

	brokerStats, statusCode, err := policy.AggregateBrokersStats(r.URL.RequestURI(), offset, limit)
	if err != nil {
		http.Error(w, "broker stats error "+err.Error(), statusCode)
		return
	}

	byte, err := json.Marshal(brokerStats)
	if err != nil {
		http.Error(w, "marshalling broker stats error "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(byte)
	return
}

// TopicProxyHandler enforces the number of topic based on the plan type
func TopicProxyHandler(w http.ResponseWriter, r *http.Request) {
	limitEnforceProxyHandler(w, r, policy.TenantManager.EvaluateAlwaysSuccessful)
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
		DirectBrokerProxyHandler(w, r)
		return
	}
	vars := mux.Vars(r)
	if tenant, ok := vars["tenant"]; ok {
		if ok, err := eval(tenant); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		} else if ok {
			DirectBrokerProxyHandler(w, r)
		} else {
			http.Error(w, "over the quota limit", http.StatusPaymentRequired)
		}
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// FunctionStatusHandler returns a function's status including worker ID as the FunctionLogHandler sees
func FunctionStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	namespace, ok2 := vars["namespace"]
	funcName, ok3 := vars["function"]
	if !(ok && ok2 && ok3) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	funcType, ok := logclient.ReadFunctionMap(tenant + namespace + funcName)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	responseBody, err := json.Marshal(funcType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(responseBody)
}

// FunctionLogsHandler responds with the function logs
func FunctionLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	namespace, ok2 := vars["namespace"]
	funcName, ok3 := vars["function"]
	instance := 0 // default instance is 0
	if instanceStr, ok4 := vars["instance"]; ok4 {
		var err error
		instance, err = strconv.Atoi(instanceStr)
		if err != nil {
			http.Error(w, "invalid instance name", http.StatusBadRequest)
			return
		}
	}
	log.WithField("app", "FunctionLogHandler").Infof("function path %s, %s, %s, instance %d", tenant, namespace, funcName, instance)
	if !(ok && ok2 && ok3) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var reqObj logclient.FunctionLogRequest
	u, _ := url.Parse(r.URL.String())
	params := u.Query()
	reqObj.BackwardPosition = int64(queryParamInt(params, "backwardpos", 0))
	reqObj.ForwardPosition = int64(queryParamInt(params, "forwardpos", 0))
	reqObj.Bytes = int64(queryParamInt(params, "bytes", 2400))
	log.WithField("app", "FunctionLogHandler").Infof("function log query params %v", reqObj)
	if reqObj.BackwardPosition > 0 && reqObj.ForwardPosition > 0 {
		http.Error(w, "backwardpos and forwardpos cannot be specified at the same time", http.StatusBadRequest)
		return
	}
	if reqObj.Bytes < 0 {
		http.Error(w, "bytes cannot be a negative value", http.StatusBadRequest)
		return
	}
	workerID := ""
	if strs, ok := params["workerid"]; ok {
		workerID = strs[0]
	}

	clientRes, err := logclient.GetFunctionLog(tenant+namespace+funcName, workerID, instance, reqObj)
	if err != nil {
		if err == logclient.ErrNotFoundFunction || strings.HasSuffix(err.Error(), "no such file or directory") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "log server returned "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// fmt.Printf("pos %d, %d\n", clientRes.BackwardPosition, clientRes.ForwardPosition)
	jsonResponse, err := json.Marshal(clientRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if clientRes.Logs == "" {
		w.WriteHeader(http.StatusNoContent)
	}
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
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if util.StrContains(util.SuperRoles, tenant) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metrics.AllNamespaceMetrics()))
		return
	}

	//TODO: disable the feature since the backend database has to populated
	/*if !policy.TenantManager.EvaluateFeatureCode(tenant, policy.BrokerMetrics) {
		http.Error(w, "", http.StatusForbidden)
	}
	*/
	tenantFederatedPrometheus(tenant, w)
}

func tenantFederatedPrometheus(tenant string, w http.ResponseWriter) {
	data := metrics.FilterFederatedMetrics(tenant)
	if len(data) > 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(data))
	} else if policy.IsTenant(tenant) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// PulsarFederatedDebugPrometheusHandler is for superrole to get individual tenant metrics
func PulsarFederatedDebugPrometheusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, _ := vars["tenant"]
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	tenantFederatedPrometheus(tenant, w)
}

// TenantUsageHandler returns tenant usage
func TenantUsageHandler(w http.ResponseWriter, r *http.Request) {
	var usages []metrics.Usage
	var err error
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	if ok {
		usages, err = metrics.GetTenantNamespacesUsage(tenant)
	} else {
		usages, err = metrics.GetTenantsUsage()
	}
	if err != nil {
		log.Errorf("failed to get tenant usage %s", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(usages)
	if err != nil {
		log.Errorf("marshal tenant usage error %s", err.Error())
		http.Error(w, "failed to marshal tenant usage data", http.StatusInternalServerError)
		return
	}
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

	// body specifies a list of must required topic,
	// the handler makes extra calls to retreive those stats if they are not in the cache
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var topicList []string
	err := decoder.Decode(&topicList)
	if err != nil && err != io.EOF {
		util.ResponseErrorJSON(err, w, http.StatusUnprocessableEntity)
		return
	}

	totalSize, newOffset, topics := policy.PaginateTopicStats(tenant, offset, pageSize, topicList)
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

// GroupTopicsByNamespaceHandler groups topics under a tenant's namespace
func GroupTopicsByNamespaceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenant, ok := vars["tenant"]
	if !ok {
		http.Error(w, "missing tenant name", http.StatusUnprocessableEntity)
		return
	}
	topics, length := policy.CountTopics(tenant)
	if length < 0 {
		w.WriteHeader(http.StatusNotFound)
	} else if length == 0 {
		w.WriteHeader(http.StatusNoContent)
	} else {
		data, err := json.Marshal(topics)
		if err != nil {
			http.Error(w, "failed to marshal cached topics data", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
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
		tenants, err := getTenantNameList()
		if err != nil {
			util.ResponseErrorJSON(err, w, http.StatusInternalServerError)
			return
		}
		found := false
		for _, t := range tenants {
			if t == tenant {
				newPlan, err = policy.TenantManager.GetOrCreateTenant(tenant)
				found = true
				break
			}
		}
		if !found {
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
}

// PulsarBeamGetTopicHandler gets the topic details
func PulsarBeamGetTopicHandler(w http.ResponseWriter, r *http.Request) {
	topicKey, err := route.GetTopicKey(r)
	if err != nil {
		util.ResponseErrorJSON(err, w, http.StatusUnprocessableEntity)
		return
	}

	// TODO: we may fix the problem that allows negatively look up by another tenant
	doc, err := policy.PulsarBeamManager.GetByKey(topicKey)
	if err != nil {
		log.Errorf("get topic error %v", err)
		util.ResponseErrorJSON(err, w, http.StatusNotFound)
		return
	}
	if !route.VerifySubjectBasedOnTopic(doc.TopicFullName, r.Header.Get("injectedSubs"), extractEvalTenant) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	resJSON, err := json.Marshal(doc)
	if err != nil {
		util.ResponseErrorJSON(err, w, http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(resJSON)
	}

}

// PulsarBeamUpdateTopicHandler is a wrapper around PulsarBeam Update Handler with additional tenant policy validation.
func PulsarBeamUpdateTopicHandler(w http.ResponseWriter, r *http.Request) {
	subject := r.Header.Get("injectedSubs")
	if subject == "" {
		http.Error(w, "missing subject", http.StatusUnauthorized)
		return
	}
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var doc model.TopicConfig
	err := decoder.Decode(&doc)
	if err != nil {
		util.ResponseErrorJSON(err, w, http.StatusUnprocessableEntity)
		return
	}
	if !route.VerifySubjectBasedOnTopic(doc.TopicFullName, r.Header.Get("injectedSubs"), extractEvalTenant) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if _, err = model.ValidateTopicConfig(doc); err != nil {
		util.ResponseErrorJSON(err, w, http.StatusUnprocessableEntity)
		return
	}

	id, err := policy.PulsarBeamManager.Update(&doc)
	if err != nil {
		log.Infof(err.Error())
		util.ResponseErrorJSON(err, w, http.StatusConflict)
		return
	}
	if len(id) > 1 {
		savedDoc, err := policy.PulsarBeamManager.GetByKey(id)
		if err != nil {
			util.ResponseErrorJSON(err, w, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		resJSON, err := json.Marshal(savedDoc)
		if err != nil {
			util.ResponseErrorJSON(err, w, http.StatusInternalServerError)
			return
		}
		w.Write(resJSON)
		return
	}
	util.ResponseErrorJSON(fmt.Errorf("failed to update"), w, http.StatusInternalServerError)
	return
}

// PulsarBeamDeleteTopicHandler deletes a topic
func PulsarBeamDeleteTopicHandler(w http.ResponseWriter, r *http.Request) {
	topicKey, err := route.GetTopicKey(r)
	if err != nil {
		util.ResponseErrorJSON(err, w, http.StatusUnprocessableEntity)
		return
	}

	doc, err := policy.PulsarBeamManager.GetByKey(topicKey)
	if err != nil {
		log.Errorf("failed to get topic based on key %s err: %v", topicKey, err)
		util.ResponseErrorJSON(err, w, http.StatusNotFound)
		return
	}
	if !route.VerifySubjectBasedOnTopic(doc.TopicFullName, r.Header.Get("injectedSubs"), extractEvalTenant) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	deletedKey, err := policy.PulsarBeamManager.DeleteByKey(topicKey)
	if err != nil {
		util.ResponseErrorJSON(err, w, http.StatusNotFound)
		return
	}
	resJSON, err := json.Marshal(deletedKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(resJSON)
	}
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

// this is a callback for Pulsar Beam's route.VerifySubjectBasedOnTopic
func extractEvalTenant(requiredSubject, tokenSub string) bool {
	subCase1, subCase2 := ExtractTenant(tokenSub)
	return requiredSubject == subCase1 || requiredSubject == subCase2
}

// ExtractTenant attempts to extract tenant based on delimiter `-` and `-client-`
// so that it will covercases such as 1. chris-kafkaesque-io-12345qbc
// 2. chris-kafkaesque-io-client-12345qbc
// 3. chris-kafkaesque-io
// 4. chris-kafkaesque-io-client-client-12345qbc
// 4. chris-kafkaesque-io-client-admin-12345qbc
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
	if parts[validLength-1] == "admin" {
		return case1, strings.Join(parts[:(validLength-1)], subDelimiter)
	}
	return case1, case1
}
