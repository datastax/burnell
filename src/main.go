package main

import (
	"log"
	"runtime"

	"github.com/google/gops/agent"
	"github.com/rs/cors"

	"github.com/kafkaesque-io/burnell/src/logclient"
	"github.com/kafkaesque-io/burnell/src/metrics"
	"github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/route"
	"github.com/kafkaesque-io/burnell/src/util"
	httptls "github.com/kafkaesque-io/pulsar-beam/src/util"
)

func main() {
	// runtime.GOMAXPROCS does not the container's CPU quota in Kubernetes
	// therefore, it requires to be set explicitly
	runtime.GOMAXPROCS(util.GetEnvInt("GOMAXPROCS", 1))

	// gops debug instrument
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatalf("gops instrument error %v", err)
	}

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
	log.Fatal(httptls.ListenAndServeTLS(":"+port, certFile, keyFile, handler))

}
