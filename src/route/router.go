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

	router.Path("/liveness").Methods(http.MethodGet).Name("liveness").Handler(NoAuth(Logger(http.HandlerFunc(StatusPage), "liveness")))
	router.Path("/subject/{sub}").Methods(http.MethodGet).Name("token server").Handler(SuperRoleRequired(Logger(http.HandlerFunc(TokenSubjectHandler), "token server")))
	router.Path("/metrics").Methods(http.MethodGet).Name("metrics").Handler(NoAuth(promhttp.Handler()))
	router.Path("/pulsarmetrics").Methods(http.MethodGet).Name("pulsar metrics").
		Handler(AuthVerifyJWT(http.HandlerFunc(PulsarFederatedPrometheusHandler)))

	// TODO: add two cases without tenant/namesapce and without tenant
	router.Path("/function-logs/{tenant}/{namespace}/{function}").Methods(http.MethodGet).Name("function-logs").
		Handler(NoAuth(http.HandlerFunc(FunctionLogsHandler)))
		// Handler(AuthVerifyJWT(http.HandlerFunc(FunctionLogsHandler)))

	router.PathPrefix("/admin/bookies/racks-info").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

	router.PathPrefix("/admin/broker-stats").Methods(http.MethodGet).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))
	// Exception is broker-resource-availability/{tenant}/{namespace}
	// since "org.apache.pulsar.broker.loadbalance.impl.ModularLoadManagerWrapper does not support this operation"
	// we would not support this for now

	router.PathPrefix("/admin/brokers/").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

	router.PathPrefix("/admin/clusters/").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

	// persistent topic
	router.PathPrefix("/admin/persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyJWT(http.HandlerFunc(VerifyTenantProxyHandler)))

	// TODO rate limit can be added per route basis
	router.Use(LimitRate)

	log.Println("router added")
	return router
}
