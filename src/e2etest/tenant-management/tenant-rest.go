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
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/datastax/burnell/src/policy"
)

// This is an end to end test program. It does these steps in order
// - Create a topic and webhook integration with Pulsar Beam
// - Verify the listener can reveive the event on the sink topic
// - Delete the topic configuration including webhook
// This test uses the default dev setting including cluster name,
// RSA key pair, and mongo database access.

const (
	restURL        = "http://localhost:8964/k/tenant"
	superuserToken = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJzdXBlcnVzZXIifQ.sEXIpI-VmQLE4c0zV8R-YIZLyne_191o5cnUjVP7GAPOnOffxFFJqGr-GYMvKn6XBhInV1pz0HucTO7FfEHYsU4McV4CPDZDY3QjTAAfgi1ca_sLYc-8b85rso4raCVC445tlZplXdzqxJ5Vh88GoGSz0gjzPp7Tc-Ih2dQAbU_-yG8_pByXIZFmtJ1zWm5u888Mh89-bOjTgADN1sx8lQUVmx4EIhqb9gNCY_T1p3HewcQXJD7-1yU9aRr0a1TzI1KdUBbwon2D-Irjw9zO1qj13rfGDdWfFASJdHCjAd1kZwI2cf-hiFFbIHKct9OGGAnbk0lFtJ8mWzQZgBRFUw"
)

func getEnvPanic(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Panic("missing required env " + key)
	return ""
}

func eval(val bool, verdict string) {
	if !val {
		log.Panic("Failed verdict " + verdict)
	}
}

func errNil(err error) {
	if err != nil {
		log.Panic(err)
	}
}

// returns the key of the topic
func addTenant() {
	// Create tenant plan
	tenantPlan := policy.TenantPlan{
		Name:         "tenant22",
		TenantStatus: policy.Activated,
		PlanType:     policy.StarterTier,
	}

	reqJSON, err := json.Marshal(tenantPlan)
	if err != nil {
		log.Fatal("TenantPlan marshalling error Error reading request. ", err)
	}
	log.Println("create tenantPlan REST call")
	req, err := http.NewRequest("POST", restURL+"/tenant22", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Fatal("Error reading request. ", err)
	}

	req.Header.Set("Authorization", superuserToken)

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error reading response from tenant management. ", err)
	}
	defer resp.Body.Close()

	log.Printf("post call to rest API statusCode %d", resp.StatusCode)
	eval(resp.StatusCode == 200, "expected rest api status code is 200")
}

func getTenant(key string) {
	log.Printf("get tenant with REST call with tenant %s\n", key)
	req, err := http.NewRequest(http.MethodGet, restURL+"/"+key, nil)
	errNil(err)

	req.Header.Set("Authorization", superuserToken)

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	errNil(err)
	defer resp.Body.Close()

	log.Printf("get topic %s rest API statusCode %d", key, resp.StatusCode)
	eval(resp.StatusCode == 200, "expected delete status code is 200")
}

func deleteTenant(key string) {
	log.Printf("delete tenant with REST call with key %s\n", key)
	req, err := http.NewRequest("DELETE", restURL+"/"+key, nil)
	errNil(err)

	req.Header.Set("Authorization", superuserToken)

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	errNil(err)
	defer resp.Body.Close()

	log.Printf("delete topic %s rest API statusCode %d", key, resp.StatusCode)
	eval(resp.StatusCode == 200, "expected delete status code is 200")
}

func main() {

	addTenant()
	log.Printf("add tenanted successfully")

	getTenant("tenant22")
	log.Printf("successful added and verified")
	deleteTenant("tenant22")

}
