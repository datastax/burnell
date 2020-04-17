package route

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter - create new router for HTTP routing
func NewRouter() *mux.Router {
	log.Println("init routes")

	router := mux.NewRouter().StrictSlash(true)

	// Order of routes definition matters

	router.Path("/liveness").Methods(http.MethodGet).Name("liveness").Handler(NoAuth(Logger(http.HandlerFunc(StatusPage), "liveness")))
	router.Path("/subject/{sub}").Methods(http.MethodGet).Name("token server").Handler(SuperRoleRequired(Logger(http.HandlerFunc(TokenSubjectHandler), "token server")))
	router.Path("/metrics").Methods(http.MethodGet).Name("metrics").Handler(NoAuth(promhttp.Handler()))
	router.Path("/pulsarmetrics").Methods(http.MethodGet).Name("pulsar metrics").
		Handler(AuthVerifyJWT(http.HandlerFunc(PulsarFederatedPrometheusHandler)))

	// Tenant policy management URL
	router.Path("/k/tenant/{tenant}").Methods(http.MethodGet, http.MethodDelete, http.MethodPost).Name("kafkaesque tenant management").
		Handler(SuperRoleRequired(http.HandlerFunc(TenantManagementHandler)))

	router.Path("/stats/topics/{tenant}").Methods(http.MethodGet).Name("tenant topic stats").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(TenantTopicStatsHandler)))

	// TODO: add two cases without tenant/namesapce and without tenant
	router.Path("/function-logs/{tenant}/{namespace}/{function}").Methods(http.MethodGet).Name("function-logs").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(FunctionLogsHandler)))

	// Pulsar Admin REST API proxy
	//
	// /bookies/
	router.PathPrefix("/admin/v2/bookies").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

	// /broker-stats
	router.PathPrefix("/admin/v2/broker-stats").Methods(http.MethodGet).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))
	// Exception is broker-resource-availability/{tenant}/{namespace}
	// since "org.apache.pulsar.broker.loadbalance.impl.ModularLoadManagerWrapper does not support this operation"
	// we would not support this for now

	//
	// /brokers
	//
	router.PathPrefix("/admin/v2/brokers").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

	//
	// /clusters
	//
	router.PathPrefix("/admin/v2/clusters").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// /namespaces
	// 1. list of routes that superroles are required under tenant
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/autoSubscriptionCreation").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/autoTopicCreation").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/backlogQuota").Methods(http.MethodGet, http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/bundles").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/clearBacklog").Methods(http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	// /admin/v2/namespaces/{tenant}/{namespace}/antiAffinity is also included for tenant access
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}").Methods(http.MethodGet).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	// 2. routes require superroles access
	// TODO to be confirmed
	router.PathPrefix("/admin/v2/namespaces").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// persistent topic
	//
	router.PathPrefix("/admin/v2/persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	// /admin/v2/persistent/{tenant}/{namespace}/partitioned

	// non-persistent topic
	router.PathPrefix("/admin/v2/non-persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	//
	// /resource-quotas
	//
	router.PathPrefix("/admin/v2/resource-quotas").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// /schemas
	//
	router.PathPrefix("/admin/v2/schemas").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	//
	// /tenants
	//
	router.PathPrefix("/admin/v2/tenants").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	// TODO rate limit can be added per route basis
	router.Use(LimitRate)

	log.Println("router added")
	return router
}
