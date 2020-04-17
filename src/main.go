package main

import (
	"log"
	"net/http"

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
	log.Printf("start serv on port %s\n", port)
	if len(certFile) > 1 && len(keyFile) > 1 {
		log.Printf("load certFile %s and keyFile %s\n", certFile, keyFile)
		log.Fatal(http.ListenAndServeTLS(":"+port, certFile, keyFile, handler))
	} else {
		log.Fatal(http.ListenAndServe(":"+port, handler))
	}

}
