package route

import (
	"encoding/json"
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
var topicStats map[string]map[string]interface{}
var topicStatsLock = sync.RWMutex{}

var statsLog = log.WithFields(log.Fields{"app": "topic stats cache"})

func brokersStatsQuery() {
	requestBrokersURL := util.SingleJoiningSlash(util.Config.ProxyURL, "brokers/"+util.Config.ClusterName)
	statsLog.Infof(requestBrokersURL)
	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestBrokersURL, nil)
	if err != nil {
		statsLog.Errorf("make http request brokers %s error %v", requestBrokersURL, err)
		return
	}
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if err != nil {
		statsLog.Errorf("GET brokers %s error %v", requestBrokersURL, err)
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		statsLog.Errorf("GET brokers %s read response body error %v", requestBrokersURL, err)
		return
	}
	var brokers []string
	if err = json.Unmarshal(body, &brokers); err != nil {
		statsLog.Errorf("GET brokers %s unmarshal response body error %v", requestBrokersURL, err)
		return
	}

	localStats := make(map[string]map[string]interface{})
	statsLog.Infof("a list of brokers - %v", brokers)
	// brokers = []string{util.Config.ProxyURL, util.Config.ProxyURL, util.Config.ProxyURL}
	for _, v := range brokers {
		stats := brokerStatsQuery(v)

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

	statsLog.Infof("total size %d, %d\n", len(localStats), len(topicStats))
}

func brokerStatsQuery(urlString string) map[string]map[string]interface{} {
	if !strings.HasPrefix(urlString, "http") {
		urlString = "http://" + urlString
	}
	topicStatsURL := util.SingleJoiningSlash(urlString, "admin/v2/broker-stats/topics")
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
	statsLog.Infof("cache size %d\n", len(stats))
	return stats
}

// CacheTopicStatsWorker is a thread to collect topic stats
func CacheTopicStatsWorker() {
	interval := time.Duration(util.GetEnvInt("StatsPullIntervalSecond", 5)) * time.Second
	go func() {
		brokersStatsQuery()
		for {
			select {
			case <-time.Tick(interval):
				brokersStatsQuery()
			}
		}
	}()
}

//
func paginateTopicStats(tenant string, offset, pageSize int) (int, int, map[string]interface{}) {
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
