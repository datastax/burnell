package route

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kafkaesque-io/burnell/src/middleware"
	. "github.com/kafkaesque-io/burnell/src/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter - create new router for HTTP routing
func NewRouter() *mux.Router {

	router := mux.NewRouter().StrictSlash(true)

	router.Path("/liveness").Methods(http.MethodGet).Name("liveness").Handler(NoAuth(Logger(http.HandlerFunc(StatusPage), "liveness")))
	router.Path("/subject").Methods(http.MethodGet).Name("token server").Handler(NoAuth(Logger(http.HandlerFunc(StatusPage), "liveness")))
	router.Path("/metrics").Methods(http.MethodGet).Name("metrics").Handler(NoAuth(promhttp.Handler()))

	router.PathPrefix("/admin/bookies/racks-info").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectProxyHandler)))

	// TODO rate limit can be added per route basis
	router.Use(middleware.LimitRate)

	log.Println("router added")
	return router
}
