//
//  Copyright (c) 2021 Datastax, Inc.
//
//  Licensed to the Apache Software Foundation (ASF) under one
//  or more contributor license agreements.  See the NOTICE file
//  distributed with this work for additional information
//  regarding copyright ownership.  The ASF licenses this file
//  to you under the Apache License, Version 2.0 (the
//  "License"); you may not use this file except in compliance
//  with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an
//  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
//  KIND, either express or implied.  See the License for the
//  specific language governing permissions and limitations
//  under the License.
//

package main

import (
	"flag"
	"os"
	"runtime"

	"github.com/apex/log"
	"github.com/google/gops/agent"
	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/datastax/burnell/src/logclient"
	"github.com/datastax/burnell/src/metrics"
	"github.com/datastax/burnell/src/policy"
	"github.com/datastax/burnell/src/route"
	"github.com/datastax/burnell/src/util"
	"github.com/datastax/burnell/src/workflow"
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
		if !util.IsStatsMode() {
			log.Infof("a full proxy mode")
			logclient.FunctionTopicWatchDog()
			policy.Initialize()
		}
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
