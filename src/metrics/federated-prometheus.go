package metrics

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/hashicorp/go-memdb"
	"github.com/kafkaesque-io/burnell/src/util"
	"github.com/prometheus/common/expfmt"
)

//Usage is the data usage per single tenant
type Usage struct {
	Name             string    `json:"name"`
	TotalMessagesIn  uint64    `json:"totalMessagesIn"`
	TotalBytesIn     uint64    `json:"totalBytesIn"`
	TotalMessagesOut uint64    `json:"totalMessagesOut"`
	TotalBytesOut    uint64    `json:"totalBytesBOut"`
	MsgInBacklog     uint64    `json:"msgInBacklog"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// TopicPerBrokerUsage is the usage for topic on each individual broker
type TopicPerBrokerUsage struct {
	ID               string    `json:"id"`
	Tenant           string    `json:"tenant"`
	Namespace        string    `json:"namespace"`
	Topic            string    `json:"topic"`
	BrokerInstance   string    `json:"brokerInstance"`
	TotalMessagesIn  uint64    `json:"totalMessagesIn"`
	TotalBytesIn     uint64    `json:"totalBytesIn"`
	TotalMessagesOut uint64    `json:"totalMessagesOut"`
	TotalBytesOut    uint64    `json:"totalBytesBOut"`
	MsgInBacklog     uint64    `json:"msgInBacklog"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

var (
	// Everything is not threadsafe. We will run concurrency only if there are too many tenants and topics to process
	// Everything runs sequentially.

	tenants     = make(map[string]bool)
	tenantsLock = sync.RWMutex{}

	cacheLock = sync.RWMutex{}
	cache     string
)

var tenantMetricNames = map[string]bool{
	"pulsar_in_bytes_total":     true,
	"pulsar_in_messages_total":  true,
	"pulsar_out_bytes_total":    true,
	"pulsar_out_messages_total": true,
	"pulsar_msg_backlog":        true,
}

var logger = log.WithFields(log.Fields{"app": "burnell,federated-prom-scraper"})

// SetCache sets the federated prom cache
func SetCache(c string) {
	cacheLock.Lock()
	cache = c
	cacheLock.Unlock()
}

// GetCache gets the federated prom cache
func GetCache() string {
	cacheLock.RLock()
	defer cacheLock.RUnlock()
	return cache
}

var usageDb *memdb.MemDB

const (
	usageDbTable = "topic-usage"
)

// Init initializes
func Init() {

	url := util.Config.FederatedPromURL
	interval := time.Duration(util.GetEnvInt("ScrapeFederatedPromIntervalSeconds", 35)) * time.Second
	logger.Infof("Federated Prometheus URL %s at interval %v\n", url, interval)
	if url != "" {
		go func(promURL string) {
			Scrape(promURL)
			for {
				select {
				case <-time.Tick(interval):
					Scrape(promURL)
				}
			}
		}(url)

		go func() {
			InitUsageDbTable()
			logger.Infof("Build tenant usage")
			for {
				select {
				case <-time.Tick(120 * time.Second):
					BuildTenantUsage()
				}
			}
		}()
	}
}

// InitUsageDbTable initializes usage db table.
func InitUsageDbTable() error {
	// Set up schema for in-memory database
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			usageDbTable: &memdb.TableSchema{
				Name: usageDbTable,
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
	usageDb, err = memdb.NewMemDB(schema)
	if err != nil {
		logger.Errorf("failed to create a new database %v", err)
	}
	return err
}

// FilterFederatedMetrics collects the metrics the subject is allowed to access
func FilterFederatedMetrics(subject string) string {
	var rc string
	scanner := bufio.NewScanner(strings.NewReader(GetCache()))

	pattern := fmt.Sprintf(`.*,namespace="%s.*`, subject)
	typeDefPattern := fmt.Sprintf(`^# TYPE .*`)
	typeDef := ""
	for scanner.Scan() {
		text := scanner.Text()
		matched, err := regexp.MatchString(typeDefPattern, text)
		if matched && err == nil {
			typeDef = text
		} else {
			matched, err = regexp.MatchString(pattern, text)
			if matched && err == nil {
				if typeDef == "" {
					rc = fmt.Sprintf("%s%s\n", rc, text)
				} else {
					rc = fmt.Sprintf("%s%s\n%s\n", rc, typeDef, text)
					typeDef = ""
				}
			}
		}
	}
	return rc
}

// AllNamespaceMetrics returns all namespace metrics on the brokers
func AllNamespaceMetrics() string {
	return GetCache()
}

// Scrape scrapes the federated prometheus endpoint
func Scrape(url string) {
	client := &http.Client{Timeout: 600 * time.Second}

	// All prometheus jobs
	// req, err := http.NewRequest("GET", url+"/?match[]={__name__=~\"..*\"}", nil)
	req, err := http.NewRequest("GET", url+"/?match[]={job=~\"broker\"}", nil)
	if err != nil {
		logger.Errorf("url request error %s", err.Error())
		return
	}

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		logger.Errorf("broker stats collection error %s", err.Error())
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	c := string(bodyBytes)
	SetCache(c)

	logger.Infof("prometheus url %s resp status code %d cach size %d", url, resp.StatusCode, len(c))
}

// BuildTenantUsage builds the tenant usage
func BuildTenantUsage() {
	ioReader := strings.NewReader(GetCache())
	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(ioReader)
	if err != nil {
		logger.Errorf("reading text format failed: %v", err)
		return
	}
	for label, mf := range metricFamilies {
		if _, ok := tenantMetricNames[label]; ok {
			for _, entry := range mf.GetMetric() {
				var broker, topic string
				for _, labelPair := range entry.GetLabel() {
					switch labelPair.GetName() {
					case "kubernetes_pod_name":
						broker = labelPair.GetValue()
					case "topic":
						topic = labelPair.GetValue()
					default:
					}
				}
				counter := entry.GetUntyped()
				UpdatePerBrokerTenantUsage(topic, broker, label, uint64(counter.GetValue()))
			}
		}
	}
}

// UpdatePerBrokerTenantUsage updates per broker tenant usage
func UpdatePerBrokerTenantUsage(topic, broker, label string, counter uint64) error {
	tenantName, namespace, topicName, err := util.ExtractPartsFromTopicFn(topic)
	if err != nil {
		return err
	}

	perBrokerUsage := TopicPerBrokerUsage{
		ID:             tenantName + namespace + topicName + broker + label,
		Tenant:         tenantName,
		Namespace:      namespace,
		Topic:          topicName,
		BrokerInstance: broker,
		UpdatedAt:      time.Now(),
	}

	switch label {
	case "pulsar_in_bytes_total":
		perBrokerUsage.TotalBytesIn = counter
	case "pulsar_in_messages_total":
		perBrokerUsage.TotalMessagesIn = counter
	case "pulsar_out_bytes_total":
		perBrokerUsage.TotalBytesOut = counter
	case "pulsar_out_messages_total":
		perBrokerUsage.TotalMessagesOut = counter
	case "pulsar_msg_backlog":
		perBrokerUsage.MsgInBacklog = counter
	default:
		return fmt.Errorf("incorrect lable %s", label)
	}
	txn := usageDb.Txn(true)
	txn.Insert(usageDbTable, &perBrokerUsage)
	txn.Commit()

	tenantsLock.Lock()
	tenants[tenantName] = true
	tenantsLock.Unlock()

	return nil
}

// GetTenantsUsage get all tenants usage
func GetTenantsUsage() ([]Usage, error) {
	tenantsUsage := make([]Usage, 0)
	tenantsLock.RLock()
	tenantNames := tenants
	tenantsLock.RUnlock()

	for tenantName := range tenantNames {
		if usage, err := GetTenantUsage(tenantName); err == nil {
			tenantsUsage = append(tenantsUsage, *usage)
		} else {
			return nil, err
		}
	}
	return tenantsUsage, nil
}

// GetTenantUsage get tenant's usage
func GetTenantUsage(tenant string) (*Usage, error) {
	usage := Usage{
		Name: tenant,
	}
	txn := usageDb.Txn(false)
	defer txn.Abort()

	result, err := txn.Get(usageDbTable, "tenant", tenant)
	if err != nil {
		return nil, err
	}

	for i := result.Next(); i != nil; i = result.Next() {
		p, ok := i.(*TopicPerBrokerUsage)
		if ok {
			usage.TotalBytesIn = usage.TotalBytesIn + p.TotalBytesIn
			usage.TotalMessagesIn = usage.TotalMessagesIn + p.TotalMessagesIn
			usage.TotalBytesOut = usage.TotalBytesOut + p.TotalBytesOut
			usage.TotalMessagesOut = usage.TotalMessagesOut + p.TotalMessagesOut
			usage.MsgInBacklog = usage.MsgInBacklog + p.MsgInBacklog
		}
	}

	usage.UpdatedAt = time.Now()
	return &usage, nil
}

// GetTenantNamespacesUsage get tenant's namespace usage
func GetTenantNamespacesUsage(tenant string) ([]Usage, error) {
	// key is tenant and namespace concatenated
	tnamespaces := make(map[string]Usage)
	txn := usageDb.Txn(false)
	defer txn.Abort()

	result, err := txn.Get(usageDbTable, "tenant", tenant)
	if err != nil {
		return nil, err
	}

	for i := result.Next(); i != nil; i = result.Next() {
		p, ok := i.(*TopicPerBrokerUsage)
		if ok {
			key := tenant + "/" + p.Namespace
			usage, exists := tnamespaces[key]
			if !exists {
				usage = Usage{
					Name:      key,
					UpdatedAt: time.Now(),
				}
			}
			usage.TotalBytesIn = usage.TotalBytesIn + p.TotalBytesIn
			usage.TotalMessagesIn = usage.TotalMessagesIn + p.TotalMessagesIn
			usage.TotalBytesOut = usage.TotalBytesOut + p.TotalBytesOut
			usage.TotalMessagesOut = usage.TotalMessagesOut + p.TotalMessagesOut
			usage.MsgInBacklog = usage.MsgInBacklog + p.MsgInBacklog

			tnamespaces[key] = usage
		}
	}

	r := make([]Usage, 0, len(tnamespaces))
	for _, v := range tnamespaces {
		r = append(r, v)
	}

	return r, nil
}
