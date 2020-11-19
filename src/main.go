package main

import (
	"flag"
	"os"
	"runtime"

	"github.com/apex/log"
	"github.com/google/gops/agent"
	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/kafkaesque-io/burnell/src/logclient"
	"github.com/kafkaesque-io/burnell/src/metrics"
	"github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/route"
	"github.com/kafkaesque-io/burnell/src/util"
	"github.com/kafkaesque-io/burnell/src/workflow"
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

	modePtr := flag.String("mode", util.Proxy, "process running mode: proxy(default), init, healer")
	flag.Parse()
	mode := util.AssignString(os.Getenv("ProcessMode"), *modePtr)
	log.Warnf("process running mode %s", mode)

	util.Init(&mode)
	config := util.GetConfig()

	var router *mux.Router
	if util.IsInitializer(&mode) {
		log.Infof("initiliazer")
		// run once for initialization and exit
		workflow.ConfigKeysJWTs(true)
		return
	} else if util.IsHealer(&mode) {
		router = route.HealerRouter()
		workflow.ConfigKeysJWTs(false)
	} else { //default proxy mode
		route.Init()
		metrics.Init()

		router = route.NewRouter()
		logclient.FunctionTopicWatchDog()
		policy.Initialize()
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8080"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"Authorization", "PulsarTopicUrl"},
	})

	handler := c.Handler(router)

	certFile := util.GetConfig().CertFile
	keyFile := util.GetConfig().KeyFile
	port := util.AssignString(config.PORT, "8080")
	err := httptls.ListenAndServeTLS(":"+port, certFile, keyFile, handler)
	if err != nil {
		log.Fatal(err.Error())
	}

}
