package main

import (
	"log"

	"github.com/rs/cors"

	"github.com/kafkaesque-io/burnell/src/logclient"
	"github.com/kafkaesque-io/burnell/src/metrics"
	"github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/route"
	"github.com/kafkaesque-io/burnell/src/util"
)

func main() {
	util.Init()
	route.Init()
	metrics.Init()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8080"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"Authorization", "PulsarTopicUrl"},
	})

	router := route.NewRouter()

	handler := c.Handler(router)
	config := util.GetConfig()

	logclient.FunctionTopicWatchDog()
	policy.Initialize()
	certFile := util.GetConfig().CertFile
	keyFile := util.GetConfig().KeyFile
	port := util.AssignString(config.PORT, "8080")
	log.Fatal(route.ListenAndServeTLS(":"+port, certFile, keyFile, handler))

}
