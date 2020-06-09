package policy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/hashicorp/go-memdb"
	"github.com/kafkaesque-io/burnell/src/util"
)

var statsLog = log.WithFields(log.Fields{"app": "broker stats cache"})

var topicStatsDB *memdb.MemDB

const (
	topicStatsDBTable = "topic-stats"
)

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

// TopicStats is the usage for topic on each individual broker
type TopicStats struct {
	ID        string      `json:"id"` // ID is the topic fullname
	Tenant    string      `json:"tenant"`
	Namespace string      `json:"namespace"`
	Topic     string      `json:"topic"`
	Data      interface{} `json:"data"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// InitTopicStatsDB initializes topicStats in-memory database
func InitTopicStatsDB() error {
	// Set up schema for in-memory database
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			topicStatsDBTable: &memdb.TableSchema{
				Name: topicStatsDBTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": &memdb.IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "ID"},
					},
					"tenant": &memdb.IndexSchema{
						Name:    "tenant",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "Tenant"},
					},
					"namespace": &memdb.IndexSchema{
						Name:    "namespace",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "Namespace"},
					},
					"topic": &memdb.IndexSchema{
						Name:    "topic",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "Topic"},
					},
				},
			},
		},
	}
	var err error
	topicStatsDB, err = memdb.NewMemDB(schema)
	if err != nil {
		statsLog.Errorf("failed to create a new topicStats database %v", err)
	}
	return err
}

// CountTopics counts the number of topics under a tenant, returns -1 if the tenant does not exist
func CountTopics(tenant string) (map[string][]string, int) {
	namespaces := make(map[string][]string)

	txn := topicStatsDB.Txn(false)
	defer txn.Abort()

	if result, err := txn.Get(topicStatsDBTable, "tenant", tenant); err == nil {
		size := 0
		for i := result.Next(); i != nil; i = result.Next() {
			topicInfo, ok := i.(*TopicStats)
			if ok && time.Since(topicInfo.UpdatedAt) < 90*time.Second {
				if topics, exists := namespaces[topicInfo.Namespace]; exists {
					namespaces[topicInfo.Namespace] = append(topics, topicInfo.ID)
				} else {
					namespaces[topicInfo.Namespace] = []string{topicInfo.ID}
				}
			}
			size++
		}
		return namespaces, size
	}

	return namespaces, -1
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
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		statsLog.Errorf("GET brokers %s error %v", requestBrokersURL, err)
		return []string{}
	}

	if response.StatusCode != http.StatusOK {
		statsLog.Errorf("GET brokers %s response status code %d", requestBrokersURL, response.StatusCode)
		return []string{}
	}

	body, err := ioutil.ReadAll(response.Body)
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
	// brokers = []string{util.Config.ProxyURL, util.Config.ProxyURL, util.Config.ProxyURL}
	for _, v := range brokers {
		brokerStatsTopicsQuery(v)
	}
}

func brokerStatsTopicsQuery(urlString string) error {
	if !strings.HasPrefix(urlString, "http") {
		urlString = "http://" + urlString
	}
	topicStatsURL := util.SingleJoinSlash(urlString, "admin/v2/broker-stats/topics")
	statsLog.Debugf(" proxy request route is %s\n", topicStatsURL)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, topicStatsURL, nil)
	if err != nil {
		statsLog.Errorf("make http request %s error %v", topicStatsURL, err)
		return err
	}
	newRequest.Header.Add("user-agent", "burnell")
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		statsLog.Errorf("make http request %s error %v", topicStatsURL, err)
		return err
	}

	if response.StatusCode != http.StatusOK {
		statsLog.Errorf("GET broker topic stats %s response status code %d", topicStatsURL, response.StatusCode)
		return err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		statsLog.Errorf("GET broker topic stats request %s error %v", topicStatsURL, err)
		return err
	}

	// tenant's namespace/bundle hash/persistent/topicFullName
	var result map[string]map[string]map[string]map[string]interface{}
	if err = json.Unmarshal(body, &result); err != nil {
		statsLog.Errorf("GET broker topic stats request %s unmarshal error %v", topicStatsURL, err)
		return err
	}

	// key is tenant, value is partition topic name
	var partitionTopicNames = make(map[string]string)

	var namespaces = make(map[string]bool)
	var namespaceTopics []string
	var allTopics = make(map[string]bool)

	for k, v := range result {
		tenant := strings.Split(k, "/")[0]
		statsLog.Debugf("namespace %s tenant %s", k, tenant)

		for bundleKey, v2 := range v {
			statsLog.Debugf("  bundle %s", bundleKey)
			for persistentKey, v3 := range v2 {
				statsLog.Debugf("    %s key", persistentKey)
				for topicFn, v4 := range v3 {
					statsLog.Debugf("      topic name %s", topicFn)
					topicInfo := TopicStats{
						ID:        topicFn,
						Tenant:    tenant,
						Namespace: k,
						UpdatedAt: time.Now(),
						Data:      v4,
					}
					if partitionName, isPartitionTopic := IsPartitionTopic(topicFn); isPartitionTopic {
						partitionTopicNames[tenant] = partitionName
					}
					if parts := strings.Split(k, "/"); len(parts) == 2 && parts[0] != "public" {
						namespaces[k] = util.IsPersistentTopic(topicFn)
					}
					allTopics[topicFn] = true

					txn := topicStatsDB.Txn(true)
					txn.Insert(topicStatsDBTable, &topicInfo)
					txn.Commit()
				}
			}
		}
	}

	for ns, isPartitioned := range namespaces {
		if topics, err := getTopicsFromNamespace(ns, isPartitioned); err == nil {
			namespaceTopics = append(namespaceTopics, topics...)
		}
	}

	var missingTopics []string
	for _, tn := range namespaceTopics {
		if _, ok := allTopics[tn]; !ok {
			missingTopics = append(missingTopics, tn)
		}
	}

	for _, tName := range missingTopics {
		if data, err := getSingleTopicStats(tName, false); err == nil {
			var ns string
			tenant, namespace, _, err := util.ExtractPartsFromTopicFn(tName)
			if err == nil {
				ns = tenant + "/" + namespace
			}
			topicInfo := TopicStats{
				ID:        tName,
				Tenant:    tenant,
				Namespace: ns,
				UpdatedAt: time.Now(),
				Data:      data,
			}

			txn := topicStatsDB.Txn(true)
			txn.Insert(topicStatsDBTable, &topicInfo)
			txn.Commit()
		}
	}

	// make separate request for partition topic for aggrgated stats
	for tenantKey, name := range partitionTopicNames {
		if data, err := getSingleTopicStats(name, true); err == nil {
			topicInfo := TopicStats{
				ID:        util.PartitionPrefix + name,
				Tenant:    tenantKey,
				Namespace: "partition topics across namespaces",
				UpdatedAt: time.Now(),
				Data:      data,
			}

			txn := topicStatsDB.Txn(true)
			txn.Insert(topicStatsDBTable, &topicInfo)
			txn.Commit()
		}
	}

	return nil
}

func getTopicsFromNamespace(path string, isPersistent bool) ([]string, error) {
	paths := "admin/v2/persistent/" + path
	if !isPersistent {
		paths = "admin/v2/non-persistent/" + path
	}
	requestBrokersURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, paths)
	newRequest, err := http.NewRequest(http.MethodGet, requestBrokersURL, nil)
	if err != nil {
		statsLog.Errorf("make http request a single topic stats %s error %v", requestBrokersURL, err)
		return nil, err
	}
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		statsLog.Errorf("GET topics under namespace %s error %v", requestBrokersURL, err)
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		statsLog.Errorf("GET topics under namespace %s response status code %d", requestBrokersURL, response.StatusCode)
		return nil, fmt.Errorf("GET topics under namesapce response status code %d", response.StatusCode)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		statsLog.Errorf("GET topics under namespace %s read response body error %v", requestBrokersURL, err)
		return nil, err
	}
	var topics []string
	if err = json.Unmarshal(body, &topics); err != nil {
		statsLog.Errorf("GET topics under namespace %s unmarshal response body error %v", requestBrokersURL, err)
		return nil, err
	}
	return topics, nil
}

// getSingleTopicStats gets aggregated partition topic stats
func getSingleTopicStats(topicFullname string, isPartitionTopic bool) (interface{}, error) {
	tenant, ns, topic, err := util.ExtractPartsFromTopicFn(topicFullname)
	if err != nil {
		return nil, err
	}
	statsRoute := util.ConditionAssign(isPartitionTopic, "/partitioned-stats", "/stats")
	topicType := util.ConditionAssign(strings.HasPrefix(topicFullname, "persistent://"), "persistent/", "non-persistent/")
	paths := "admin/v2/" + topicType + tenant + "/" + ns + "/" + topic + statsRoute

	requestBrokersURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, paths)

	client := *http.DefaultClient
	// keep authorization header for the redirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if len(via) == 0 {
			return nil
		}
		for attr, val := range via[0].Header {
			if _, ok := req.Header[attr]; !ok {
				req.Header[attr] = val
			}
		}
		return nil
	}

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestBrokersURL, nil)
	if err != nil {
		statsLog.Errorf("make http request a single topic stats %s error %v", requestBrokersURL, err)
		return nil, err
	}
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		statsLog.Errorf("GET a single topic stats %s error %v", requestBrokersURL, err)
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		statsLog.Errorf("GET a single topic stats %s response status code %d", requestBrokersURL, response.StatusCode)
		return nil, fmt.Errorf("GET a single topic stats response status code %d", response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		statsLog.Errorf("GET a single topic stats %s read response body error %v", requestBrokersURL, err)
		return nil, err
	}

	var topicStats interface{}
	if err = json.Unmarshal(body, &topicStats); err != nil {
		statsLog.Errorf("GET a single topic stats %s unmarshal response body error %v", requestBrokersURL, err)
		return nil, err
	}
	return topicStats, nil
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
func PaginateTopicStats(tenant string, offset, pageSize int, mandatoryTopics []string) (int, int, map[string]interface{}) {
	txn := topicStatsDB.Txn(false)
	result, err := txn.Get(topicStatsDBTable, "tenant", tenant)
	if err != nil {
		return -1, -1, nil
	}

	allTopics := make(map[string]interface{})
	keys := make([]string, 0)
	for i := result.Next(); i != nil; i = result.Next() {
		p, ok := i.(*TopicStats)
		if ok {
			if time.Since(p.UpdatedAt) < 90*time.Second {
				keys = append(keys, p.ID)
				allTopics[p.ID] = p.Data
			} else {
				txn.DeleteAll(topicStatsDBTable, "id", p.ID)
			}
		}
	}
	txn.Abort()

	// make sure mandatory topics stats exists
	for _, name := range mandatoryTopics {
		if _, ok := allTopics[name]; !ok {
			tName, isPartitioned := util.ParsePartitionTopicName(name)
			if data, err := getSingleTopicStats(tName, isPartitioned); err == nil {
				var ns string
				if _, namespace, _, err := util.ExtractPartsFromTopicFn(tName); err == nil {
					ns = tenant + "/" + namespace
				}
				ns = util.ConditionAssign(isPartitioned, "partition topics across namespaces", ns)

				topicInfo := TopicStats{
					ID:        util.PartitionPrefix + name,
					Tenant:    tenant,
					Namespace: ns,
					UpdatedAt: time.Now(),
					Data:      data,
				}

				txn := topicStatsDB.Txn(true)
				txn.Insert(topicStatsDBTable, &topicInfo)
				txn.Commit()

			}
		}
	}

	totalSize := len(keys)

	// returns no data since what's asked is already over the total size
	if offset > totalSize {
		return -1, -1, nil
	}

	newMap := make(map[string]interface{})
	sort.Strings(keys)
	newOffset := offset + pageSize
	if newOffset > totalSize {
		newOffset = totalSize
	}
	reqKeys := keys[offset:newOffset]
	for _, v := range reqKeys {
		newMap[v] = allTopics[v]
	}
	return totalSize, newOffset, newMap
}

// AggregateBrokersStats aggregates all brokers' statistics
func AggregateBrokersStats(subRoute string, offset, limit int) (BrokersStats, int, error) {
	if offset < 0 || limit < 0 {
		return BrokersStats{}, http.StatusUnprocessableEntity, fmt.Errorf("offset or limit cannot be negative")
	}
	brokers := GetBrokers()
	// brokers := []string{util.Config.BrokerProxyURL, util.Config.BrokerProxyURL, util.Config.BrokerProxyURL}
	statsLog.Infof("broker %v", brokers)

	total := len(brokers)
	if offset >= total {
		return BrokersStats{}, http.StatusUnprocessableEntity, fmt.Errorf("offset %d cannot reconcile with the total number of brokers %d", offset, total)
	}

	newOffset := offset + limit
	if newOffset == 0 || newOffset > total {
		newOffset = total
	} else if limit == 0 {
		return BrokersStats{}, http.StatusUnprocessableEntity, fmt.Errorf("limit must be greater than 0")
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
				return resp, http.StatusOK, nil
			}
		case <-time.Tick(BrokerTimeoutSecond):
			statsLog.Errorf("timeout on brokers stats response")
			resp.Data = brokerStats
			return resp, http.StatusInternalServerError, fmt.Errorf("broker stats time out by server")
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
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		statsLog.Errorf("GET broker topic stats make http request %s error %v", brokerStatsURL, err)
		respChan <- brokerStats
		return
	}

	if response.StatusCode != http.StatusOK {
		statsLog.Errorf("GET broker topic stats %s response status code %d", brokerStatsURL, response.StatusCode)
		respChan <- brokerStats
		return
	}

	body, err := ioutil.ReadAll(response.Body)
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

// IsPartitionTopic verifies if the topic is a partition topic.
func IsPartitionTopic(name string) (string, bool) {
	r := regexp.MustCompile(`^[a-z].*-partition-[0-9]{1,5}$`)
	if r.MatchString(name) {
		parts := strings.Split(name, "-")
		return strings.Join(parts[:len(parts)-2], "-"), true
	}
	return "", false
}
