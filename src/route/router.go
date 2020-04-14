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

	// TODO: add two cases without tenant/namesapce and without tenant
	router.Path("/function-logs/{tenant}/{namespace}/{function}").Methods(http.MethodGet).Name("function-logs").
		Handler(NoAuth(http.HandlerFunc(FunctionLogsHandler)))
		// Handler(AuthVerifyJWT(http.HandlerFunc(FunctionLogsHandler)))

	// Pulsar Admin REST API proxy
	//
	// /bookies/
	router.PathPrefix("/admin/bookies").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

		// /broker-stats
	router.PathPrefix("/admin/broker-stats").Methods(http.MethodGet).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))
	// Exception is broker-resource-availability/{tenant}/{namespace}
	// since "org.apache.pulsar.broker.loadbalance.impl.ModularLoadManagerWrapper does not support this operation"
	// we would not support this for now

	//
	// /brokers
	//
	router.PathPrefix("/admin/brokers").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

		//
	// /clusters
	//
	router.PathPrefix("/admin/clusters").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

		//
	// /namespaces
	// 1. list of routes that superroles are required under tenant
	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}/autoSubscriptionCreation").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantNamespaceCachedProxyHandler)))
	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}/autoTopicCreation").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantNamespaceCachedProxyHandler)))
	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}/backlogQuota").Methods(http.MethodGet, http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantNamespaceCachedProxyHandler)))
	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}/bundles").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantNamespaceCachedProxyHandler)))

	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}/clearBacklog").Methods(http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	// /admin/namespaces/{tenant}/{namespace}/antiAffinity is also included for tenant access
	router.PathPrefix("/admin/namespaces/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	router.PathPrefix("/admin/namespaces/{tenant}").Methods(http.MethodGet).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	// 2. routes require superroles access
	// TODO to be confirmed
	router.PathPrefix("/admin/namespaces").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// persistent topic
	//
	router.PathPrefix("/admin/persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))
	// /admin/persistent/{tenant}/{namespace}/partitioned

	// non-persistent topic
	router.PathPrefix("/admin/non-persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	//
	// /resource-quotas
	//
	router.PathPrefix("/admin/schemas").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	//
	// /schemas
	//
	router.PathPrefix("/admin/schemas").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	//
	// /tenants
	//
	router.PathPrefix("/admin/tenants").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(VerifyTenantCachedProxyHandler)))

	// TODO rate limit can be added per route basis
	router.Use(LimitRate)

	log.Println("router added")
	return router
}
