package policy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/kafkaesque-io/burnell/src/util"
)

// topicStats is the master cache for topics
// the first key is tenant, the second key is topic full name
var topicStats = make(map[string]map[string]interface{})
var topicStatsLock = sync.RWMutex{}

var statsLog = log.WithFields(log.Fields{"app": "broker stats cache"})

// BrokerStats is per broker statistics
type BrokerStats struct {
	Broker string      `json:"broker"`
	Data   interface{} `json:"data"`
}

// BrokersStats is the json response object for REST API
type BrokersStats struct {
	Total  int           `json:"total"`
	Offset int           `json:"offset"`
	Data   []BrokerStats `json:"data"`
}

// CountTopics counts the number of topics under a tenant, returns -1 if the tenant does not exist
func CountTopics(tenant string) int {
	topicStatsLock.RLock()
	defer topicStatsLock.RUnlock()
	if topics, ok := topicStats[tenant]; ok {
		return len(topics)
	}
	return -1
}

// GetBrokers gets a list of broker IP or fqdn
func GetBrokers() []string {
	requestBrokersURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, "admin/v2/brokers/"+util.Config.ClusterName)
	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestBrokersURL, nil)
	if err != nil {
		statsLog.Errorf("make http request brokers %s error %v", requestBrokersURL, err)
		return []string{}
	}
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if err != nil {
		statsLog.Errorf("GET brokers %s error %v", requestBrokersURL, err)
		return []string{}
	}

	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		statsLog.Errorf("GET brokers %s read response body error %v", requestBrokersURL, err)
		return []string{}
	}
	var brokers []string
	if err = json.Unmarshal(body, &brokers); err != nil {
		statsLog.Errorf("GET brokers %s unmarshal response body error %v", requestBrokersURL, err)
		return []string{}
	}
	return sort.StringSlice(brokers)
}

func brokersStatsTopicQuery() {
	brokers := GetBrokers()
	localStats := make(map[string]map[string]interface{})
	// brokers = []string{util.Config.ProxyURL, util.Config.ProxyURL, util.Config.ProxyURL}
	for _, v := range brokers {
		stats := brokerStatsTopicsQuery(v)

		// merge stats with the master cache
		for tenant, topics := range stats {
			if v, ok := localStats[tenant]; ok {
				for topicName, topicStats := range topics {
					v[topicName] = topicStats
				}
				localStats[tenant] = v
			} else {
				localStats[tenant] = topics
			}
		}
	}

	topicStatsLock.Lock()
	topicStats = localStats
	topicStatsLock.Unlock()

	statsLog.Infof("total size %d, local topic stats size %d, broker list%v\n", len(localStats), len(topicStats), brokers)
}

func brokerStatsTopicsQuery(urlString string) map[string]map[string]interface{} {
	if !strings.HasPrefix(urlString, "http") {
		urlString = "http://" + urlString
	}
	topicStatsURL := util.SingleJoinSlash(urlString, "admin/v2/broker-stats/topics")
	statsLog.Debugf(" proxy request route is %s\n", topicStatsURL)

	stats := make(map[string]map[string]interface{})

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, topicStatsURL, nil)
	if err != nil {
		statsLog.Errorf("make http request %s error %v", topicStatsURL, err)
		return stats
	}
	newRequest.Header.Add("user-agent", "burnell")
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if err != nil {
		statsLog.Errorf("make http request %s error %v", topicStatsURL, err)
		return stats
	}

	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		statsLog.Errorf("GET broker topic stats request %s error %v", topicStatsURL, err)
		return stats
	}

	// tenant's namespace/bundle hash/persistent/topicFullName
	var result map[string]map[string]map[string]map[string]interface{}
	if err = json.Unmarshal(body, &result); err != nil {
		statsLog.Errorf("GET broker topic stats request %s unmarshal error %v", topicStatsURL, err)
		return stats
	}
	for k, v := range result {
		tenant := strings.Split(k, "/")[0]
		statsLog.Debugf("namespace %s tenant %s", k, tenant)
		topics := make(map[string]interface{})

		for bundleKey, v2 := range v {
			statsLog.Debugf("  bundle %s", bundleKey)
			for persistentKey, v3 := range v2 {
				statsLog.Debugf("    %s key", persistentKey)
				for topicFn, v4 := range v3 {
					statsLog.Debugf("      topic name %s", topicFn)
					topics[topicFn] = v4
				}
			}
		}
		stats[tenant] = topics
	}
	return stats
}

// CacheTopicStatsWorker is a thread to collect topic stats
func CacheTopicStatsWorker() {
	interval := time.Duration(util.GetEnvInt("StatsPullIntervalSecond", 5)) * time.Second
	go func() {
		brokersStatsTopicQuery()
		for {
			select {
			case <-time.Tick(interval):
				brokersStatsTopicQuery()
			}
		}
	}()
}

// PaginateTopicStats paginate topic statistics returns based on offset and page size limit
func PaginateTopicStats(tenant string, offset, pageSize int) (int, int, map[string]interface{}) {
	topicStatsLock.RLock()
	topics, ok := topicStats[tenant]
	topicStatsLock.RUnlock()
	if !ok {
		return -1, -1, nil
	}

	totalSize := len(topics)

	// returns no data since what's asked is already over the total size
	if offset > totalSize {
		return -1, -1, nil
	}

	keys := make([]string, 0)
	newMap := make(map[string]interface{})

	for k := range topics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	newOffset := offset + pageSize
	if newOffset > totalSize {
		newOffset = totalSize
	}
	reqKeys := keys[offset:newOffset]
	for _, v := range reqKeys {
		newMap[v] = topics[v]
	}
	return totalSize, newOffset, newMap
}

// AggregateBrokersStats aggregates all brokers' statistics
func AggregateBrokersStats(subRoute string, offset, limit int) (BrokersStats, error, int) {
	if offset < 0 || limit < 0 {
		return BrokersStats{}, fmt.Errorf("offset or limit cannot be negative"), http.StatusUnprocessableEntity
	}
	brokers := GetBrokers()
	// brokers := []string{util.Config.BrokerProxyURL, util.Config.BrokerProxyURL, util.Config.BrokerProxyURL}
	statsLog.Infof("broker %v", brokers)

	total := len(brokers)
	if offset >= total {
		return BrokersStats{}, fmt.Errorf("offset %d cannot reconcile with the total number of brokers %d", offset, total), http.StatusUnprocessableEntity
	}

	newOffset := offset + limit
	if newOffset == 0 || newOffset > total {
		newOffset = total
	} else if limit == 0 {
		return BrokersStats{}, fmt.Errorf("limit must be greater than 0"), http.StatusUnprocessableEntity
	}
	brokers = brokers[offset:newOffset]

	size := len(brokers) //calculate the new size

	resp := BrokersStats{
		Total:  total,
		Offset: newOffset,
	}

	var brokerStats []BrokerStats
	resultChan := make(chan BrokerStats, size)
	BrokerTimeoutSecond := time.Duration(size*2) * time.Second

	for _, broker := range brokers {
		go brokerStatsQuery(broker, subRoute, resultChan)
	}

	for {
		select {
		case result := <-resultChan:
			brokerStats = append(brokerStats, result)
			if size--; size == 0 {
				resp.Data = brokerStats
				return resp, nil, http.StatusOK
			}
		case <-time.Tick(BrokerTimeoutSecond):
			statsLog.Errorf("timeout on brokers stats response")
			resp.Data = brokerStats
			return resp, fmt.Errorf("broker stats time out by server"), http.StatusInternalServerError
		}
	}

}

func brokerStatsQuery(urlString, subRoute string, respChan chan BrokerStats) {
	brokerStats := BrokerStats{
		Broker: urlString,
	}
	if !strings.HasPrefix(urlString, "http") {
		urlString = "http://" + urlString
	}
	brokerStatsURL := util.SingleJoinSlash(urlString, subRoute)
	statsLog.Infof(" proxy request route is %s\n", brokerStatsURL)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, brokerStatsURL, nil)
	if err != nil {
		statsLog.Errorf("make http request %s error %v", brokerStatsURL, err)
		respChan <- brokerStats
		return
	}
	newRequest.Header.Add("user-agent", "burnell")
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if err != nil {
		statsLog.Errorf("make http request %s error %v", brokerStatsURL, err)
		respChan <- brokerStats
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		statsLog.Errorf("GET broker topic stats request %s error %v", brokerStatsURL, err)
		respChan <- brokerStats
		return
	}

	// tenant's namespace/bundle hash/persistent/topicFullName
	var result interface{}
	if err = json.Unmarshal(body, &result); err != nil {
		statsLog.Errorf("GET broker topic stats request %s unmarshal error %v", brokerStatsURL, err)
		respChan <- brokerStats
		return
	}
	brokerStats.Data = result
	respChan <- brokerStats
}
