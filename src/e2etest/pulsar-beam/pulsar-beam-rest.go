package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kafkaesque-io/pulsar-beam/src/model"
)

// This is an end to end test program. It does these steps in order
// - Create a topic and webhook integration with Pulsar Beam
// - Verify the listener can reveive the event on the sink topic
// - Delete the topic configuration including webhook
// This test uses the default dev setting including cluster name,
// RSA key pair, and mongo database access.

const (
	restURL      = "http://localhost:8964/pulsarbeam/v2/topic"
	pulsarToken  = "test-token"
	pulsarURL    = "pulsar://localhost:6650"
	webhookTopic = "persistent://ming-luo/ns/topic2"
	webhookURL   = "http://server.com/endpoint"
	restAPIToken = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJtaW5nLWx1by0xMjM0In0.jpaAd9N_LVm7B03EY_aTMZ1Dd3IdL_pTi4SyIojSWR4skRRjQGm0upY3CLJnHTMbnoe4-1Q6BPVwCkicV9luPTI3Nk3cSjtEmyMRtKVNuJLeeEvt7wU8fMi8pFt9EfnkDkgEz8oNzkgpz6dnUuiGKFBwsMx1u29v6lB0YK2twz6B-mOgwByKYC01nwOPqXygrGkph8t08EjbqQHyYtF5GWCAKRlkA6AVx3x6tr4r6c1tzBKUWGTh0zYN_RcxRhI6FV7kZb8UxhYGB_b_xkd8WVbyTxSGgeZF9Fg5WOLJm280h7i5c767he_qXGC_krrqpZHm2p4XVFGNOJ7p7bT5Vg"
)

func init() {
}

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
func addWebhookToDb() string {
	// Create a topic and webhook via REST
	topicConfig, err := model.NewTopicConfig(webhookTopic, pulsarURL, pulsarToken)
	errNil(err)

	log.Printf("register webhook %s\nwith pulsar %s and topic %s\n", webhookURL, pulsarURL, webhookTopic)
	wh := model.NewWebhookConfig(webhookURL)
	wh.InitialPosition = "earliest"
	wh.Subscription = "my-subscription"
	topicConfig.Webhooks = append(topicConfig.Webhooks, wh)

	if _, err = model.ValidateTopicConfig(topicConfig); err != nil {
		log.Fatal("Invalid topic config ", err)
	}

	reqJSON, err := json.Marshal(topicConfig)
	if err != nil {
		log.Fatal("Topic marshalling error Error reading request. ", err)
	}
	log.Println("create topic and webhook with REST call")
	req, err := http.NewRequest("POST", restURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Fatal("Error reading request. ", err)
	}

	req.Header.Set("Authorization", restAPIToken)

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error reading response from Beam. ", err)
	}
	defer resp.Body.Close()

	log.Printf("post call to rest API statusCode %d", resp.StatusCode)
	eval(resp.StatusCode == 201, "expected rest api status code is 201")
	return topicConfig.Key
}

func getWebhook(key string) {
	url := restURL + "/" + key
	log.Printf("get topic and webhook with REST call with url %s\n", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	errNil(err)

	req.Header.Set("Authorization", restAPIToken)

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	errNil(err)
	defer resp.Body.Close()

	log.Printf("get topic %s rest API statusCode %d", key, resp.StatusCode)
	eval(resp.StatusCode == 200, "expected GET status code is 200")
}

func deleteWebhook(key string) {
	url := restURL + "/" + key
	log.Printf("delete topic and webhook with REST call with url %s\n", url)
	req, err := http.NewRequest("DELETE", url, nil)
	errNil(err)

	req.Header.Set("Authorization", restAPIToken)

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	errNil(err)
	defer resp.Body.Close()

	log.Printf("delete topic %s rest API statusCode %d", key, resp.StatusCode)
	eval(resp.StatusCode == 200, "expected DELETE status code is 200")
}

func main() {

	key := addWebhookToDb()
	log.Printf("added webhook %s", key)

	getWebhook(key)
	log.Printf("successful added and verified")
	deleteWebhook(key)

}
