package util

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// ResponseErr - Error struct for Http response
type ResponseErr struct {
	Error string `json:"error"`
}

// NewUUID generates a random UUID according to RFC 4122
func NewUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

// JoinString joins multiple strings
func JoinString(strs ...string) string {
	var sb strings.Builder
	for _, str := range strs {
		sb.WriteString(str)
	}
	return sb.String()
}

// ResponseErrorJSON builds a Http response.
func ResponseErrorJSON(e error, w http.ResponseWriter, statusCode int) {
	response := ResponseErr{e.Error()}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(jsonResponse)
}

// ReceiverHeader parses headers for Pulsar required configuration
func ReceiverHeader(h *http.Header) (token, topicFN, pulsarURL string, err bool) {
	token = strings.TrimSpace(strings.Replace(h.Get("Authorization"), "Bearer", "", 1))
	topicFN = h.Get("TopicFn")
	pulsarURL = h.Get("PulsarUrl")
	return token, topicFN, pulsarURL, token == "" || topicFN == "" || pulsarURL == ""

}

// AssignString returns the first non-empty string
// It is equivalent the following in Javascript
// var value = val0 || val1 || val2 || default
func AssignString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// ReportError logs error
func ReportError(err error) error {
	log.Printf("error %v", err)
	return err
}

// StrContains check if a string is contained in an array of string
func StrContains(strs []string, str string) bool {
	for _, v := range strs {
		if v == str {
			return true
		}
	}
	return false
}

// GetEnvInt gets OS environment in integer format with a default if inproper value retrieved
func GetEnvInt(env string, defaultNum int) int {
	if i, err := strconv.Atoi(os.Getenv(env)); err == nil {
		return i
	}
	return defaultNum
}

// SingleJoinSlash joins two parts of url path with no double slash
func SingleJoinSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// ExtractPartsFromTopicFn extracts tenant, namespace, topic name from a full topic name
func ExtractPartsFromTopicFn(topicFn string) (string, string, string, error) {
	if !(strings.HasPrefix(topicFn, "persistent://") || strings.HasPrefix(topicFn, "non-persistent://")) {
		return "", "", "", fmt.Errorf("topic type supports persistent:// or non-persistent:// but the topic is %s", topicFn)
	}

	topicFnParts := strings.Split(topicFn, "://")
	if len(topicFnParts) != 2 {
		return "", "", "", fmt.Errorf("incorrect formated topic fullname")
	}
	parts := strings.Split(topicFnParts[1], "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("incorrect formated topic tenant namespace topic parts")
	}

	return parts[0], parts[1], parts[2], nil
}

// BytesToMegaBytesFloor converts bytes to megabytes.
// 0 bytes -> 0 MB
// Between 0 to 1 MB -> 1MB (that means one byte is 1 MB)
// over 1MB rounds down (that means 15.9MB is 15MB)
func BytesToMegaBytesFloor(bytes int64) int64 {
	if bytes == 0 {
		return 0
	}
	mb := bytes / (1000 * 1000)
	if mb > 1 {
		return mb
	}
	return 1
}

// ConditionAssign is an one line assign value based on condition
func ConditionAssign(condition bool, trueValue, falseValue string) string {
	if condition {
		return trueValue
	}
	return falseValue
}

// PartitionPrefix is the prefix for partition topics
const PartitionPrefix = "partition-"

const persistentPrefix = "persistent://"

// ParsePartitionTopicName parses topic full name to the one Pulsar regonizes
// returns a topic full name and boolean value indicate whether it is a partition topic
func ParsePartitionTopicName(topic string) (string, bool) {
	if strings.HasPrefix(topic, PartitionPrefix) {
		return strings.Replace(topic, PartitionPrefix, "", 1), true
	}
	return topic, false
}

// IsPersistentTopic returns if the topic is a persistent topic
func IsPersistentTopic(topic string) bool {
	return strings.HasPrefix(topic, persistentPrefix)
}
