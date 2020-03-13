package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/rs/cors"

	"github.com/kafkaesque-io/burnell/src/route"
	"github.com/kafkaesque-io/burnell/src/util"
)

var mode = flag.String("mode", "hybrid", "server running mode")

func main() {
	util.Init()

	flag.Parse()
	log.Println("start server mode ", *mode)
	if !util.IsValidMode(mode) {
		log.Panic("Unsupported server mode")
	}

	route.Init()

		c := cors.New(cors.Options{
			AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8080"},
			AllowCredentials: true,
			AllowedHeaders:   []string{"Authorization", "PulsarTopicUrl"},
		})

		router := route.NewRouter(mode)

		handler := c.Handler(router)
		config := util.GetConfig()
		port := util.AssignString(config.PORT, "8080")
		log.Fatal(http.ListenAndServe(":"+port, handler))

}
