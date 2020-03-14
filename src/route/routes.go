package route

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kafkaesque-io/burnell/src/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Route - HTTP Route
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
	AuthFunc    mux.MiddlewareFunc
}

// Routes list of HTTP Routes
type Routes []Route

// EndpointRoutes definition
var EndpointRoutes = Routes{
	Route{
		"token server",
		http.MethodGet,
		"/subject",
		TokenSubjectHandler,
		middleware.AuthVerifyJWT,
	},

	// this can be used as kubernetes readiness
	Route{
		"Prometeus metrics",
		http.MethodGet,
		"/metrics",
		promhttp.Handler().ServeHTTP,
		middleware.NoAuth,
	},
	Route{
		"Pulsar Admin",
		"",
		"/admin",
		AdminProxyHandler,
		middleware.AuthHeaderRequired,
	},

	// this is for kubernetes liveness
	Route{
		"status",
		"GET",
		"/liveness",
		StatusPage,
		middleware.AuthHeaderRequired,
	},
}
