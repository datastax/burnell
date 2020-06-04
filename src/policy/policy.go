package policy

import (
	"log"
	"strings"
	"time"

	"github.com/kafkaesque-io/burnell/src/util"
	"github.com/kafkaesque-io/pulsar-beam/src/db"
)

// TenantStatus can be used for tenant status
type TenantStatus int

// state machine of tenant status
const (
	// Reserved makes sure the status always starting with 1
	Reserved0 TenantStatus = iota
	// Activated is the only active state
	Activated
	// Deactivated is the beginning state
	Deactivated
	// Suspended is the state between Activated and Deleted
	Suspended
	// Deleted is the end of state
	Deleted
)

const (
	// FreeTier is free tier tenant policy
	FreeTier = "free"
	// StarterTier is the starter tier
	StarterTier = "starter"
	// ProductionTier is the production tier
	ProductionTier = "production"
	// DedicatedTier is the decicated tier
	DedicatedTier = "dedicated"
	// PrivateTier is the private tier
	PrivateTier = "private"
)

const (
	// FeatureAllEnabled indicates all features are enabled
	FeatureAllEnabled = "all-enabled"
	// FeatureAllDisabled indicates all features are disabled
	FeatureAllDisabled = "all-disabled"
	// BrokerMetrics is the feature to expose broker prometheus metrics
	BrokerMetrics = "broker-metrics"
	// InfiniteMessageRetention is the feature for infinite message retention
	InfiniteMessageRetention = "infinite-message-retention"
)

// PlanPolicy is the tenant policy
// this allows additional customization and feature licensing
type PlanPolicy struct {
	Name                 string        `json:"name"`
	NumOfTopics          int           `json:"numOfTopics"`
	NumOfNamespaces      int           `json:"numOfNamespaces"`
	MessageHourRetention int           `json:"messageHourRetention"` //Golang only allows json unmarshal to ns therefore conversion is required to hours
	MessageRetention     time.Duration `json:"messageRetention"`
	NumOfProducers       int           `json:"numofProducers"`
	NumOfConsumers       int           `json:"numOfConsumers"`
	Functions            int           `json:"functions"`
	FeatureCodes         string        `json:"featureCodes"`
	Reserved0            string        `json:"reserved0"`
	Reserved1            string        `json:"reserved1"`
}

// TenantPlan is the tenant plan information stored in the database
type TenantPlan struct {
	Name         string       `json:"name"`
	TenantStatus TenantStatus `json:"tenantStatus"`
	Org          string       `json:"org"`
	Users        string       `json:"users"`
	PlanType     string       `json:"planType"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	Policy       PlanPolicy   `json:"policy"`
	Audit        string       `json:"audit"`
}

// PlanPolicies struct
type PlanPolicies struct {
	FreePlan       PlanPolicy
	StarterPlan    PlanPolicy
	ProductionPlan PlanPolicy
	DedicatedPlan  PlanPolicy
	PrivatePlan    PlanPolicy
}

// TenantPolicyEvaluator evaluates the tenant management policy
type TenantPolicyEvaluator interface {
	Conn(hosts string) error
	GetPlanPolicy(tenantName string) PlanPolicy
	Evaluate(tenantName string) error
}

// TenantPlanPolicies is all plan policies
var TenantPlanPolicies = PlanPolicies{
	FreePlan: PlanPolicy{
		Name:                 FreeTier,
		NumOfTopics:          5,
		NumOfNamespaces:      1,
		MessageRetention:     2 * 24 * time.Hour,
		MessageHourRetention: 2 * 24,
		NumOfProducers:       3,
		NumOfConsumers:       5,
		Functions:            1,
		FeatureCodes:         FeatureAllDisabled,
	},
	StarterPlan: PlanPolicy{
		Name:                 StarterTier,
		NumOfTopics:          20,
		NumOfNamespaces:      2,
		MessageRetention:     7 * 24 * time.Hour,
		MessageHourRetention: 7 * 24,
		NumOfProducers:       30,
		NumOfConsumers:       50,
		Functions:            10,
		FeatureCodes:         FeatureAllDisabled,
	},
	ProductionPlan: PlanPolicy{
		Name:                 ProductionTier,
		NumOfTopics:          100,
		NumOfNamespaces:      6,
		MessageRetention:     14 * 24 * time.Hour,
		MessageHourRetention: 14 * 24,
		NumOfProducers:       60,
		NumOfConsumers:       100,
		Functions:            20,
		FeatureCodes:         FeatureAllDisabled,
	},
	DedicatedPlan: PlanPolicy{
		Name:                 DedicatedTier,
		NumOfTopics:          1000,
		NumOfNamespaces:      500,
		MessageRetention:     21 * 24 * time.Hour,
		MessageHourRetention: 21 * 24,
		NumOfProducers:       300,
		NumOfConsumers:       500,
		Functions:            30,
		FeatureCodes:         FeatureAllDisabled,
	},
	PrivatePlan: PlanPolicy{
		Name:                 PrivateTier,
		NumOfTopics:          5000,
		NumOfNamespaces:      1000,
		MessageRetention:     28 * 24 * time.Hour,
		MessageHourRetention: 28 * 24,
		NumOfProducers:       -1,
		NumOfConsumers:       -1,
		Functions:            -1,
		FeatureCodes:         FeatureAllEnabled,
	},
}

func getPlanPolicy(plan string) *PlanPolicy {
	switch plan {
	case FreeTier:
		return &TenantPlanPolicies.FreePlan
	case StarterTier:
		return &TenantPlanPolicies.StarterPlan
	case ProductionTier:
		return &TenantPlanPolicies.ProductionPlan
	case DedicatedTier:
		return &TenantPlanPolicies.DedicatedPlan
	case PrivateTier:
		return &TenantPlanPolicies.PrivatePlan
	default:
		return nil
	}
}

// TenantManager is the global object to manage the Tenant REST API
var TenantManager TenantPolicyHandler

// PulsarBeamManager is the global object the manage the Pulsar Beam topic
var PulsarBeamManager db.PulsarHandler

// Initialize initializes database
func Initialize() {
	if err := TenantManager.Setup(); err != nil {
		log.Fatal(err)
	}

	if util.GetConfig().PulsarBeamTopic != "" {

		PulsarBeamManager = db.PulsarHandler{}
		PulsarBeamManager.PulsarURL = util.GetConfig().PulsarURL
		PulsarBeamManager.TopicName = util.GetConfig().PulsarBeamTopic
		PulsarBeamManager.PulsarToken = util.GetConfig().PulsarToken
		if err := PulsarBeamManager.Init(); err != nil {
			log.Fatal(err)
		}
	}

	if err := InitTopicStatsDB(); err != nil {
		panic(err)
	}
	CacheTopicStatsWorker()
}

// Init is called at bootstrap to build feature codes
func Init() {

}

// IsFeatureSupported checks if the feature is supported
func IsFeatureSupported(feature, featureCodes string) bool {
	return featureCodes == FeatureAllEnabled || util.StrContains(strings.Split(featureCodes, ","), feature)
}
