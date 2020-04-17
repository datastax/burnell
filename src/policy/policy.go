package policy

import (
	"log"
	"net/http"
	"time"

	"github.com/kafkaesque-io/burnell/src/util"
	"github.com/patrickmn/go-cache"
)

// TenantPolicyMap has tenant name as key, tenant policy as value
var TenantPolicyMap = cache.New(15*time.Minute, 3*time.Hour)

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

// FeatureStatus uses 1 or 2 to avoid 0 as it is the default value instantiated with
type FeatureStatus int

const (
	// Reserved makes sure the status always starting with 1
	Reserved FeatureStatus = iota
	// Enabled the feautre is enabled
	Enabled
	// Disabled the feautre is disabled
	Disabled
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

const nsToHour = 1000 * 1000 * 1000 * 3600

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
	BrokerMetrics        FeatureStatus `json:"brokerMetrics"`
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
		BrokerMetrics:        Disabled,
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
		BrokerMetrics:        Disabled,
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
		BrokerMetrics:        Disabled,
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
		BrokerMetrics:        Disabled,
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
		BrokerMetrics:        Enabled,
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

// UpdateCache updates the tenant policy map
func UpdateCache(tenant string, plan PlanPolicy) {
	TenantPolicyMap.Add(tenant, plan, cache.DefaultExpiration)
}

// EvalNamespaceAdminAPI evaluate tenant's namespace administration permission
func EvalNamespaceAdminAPI(r *http.Request, subject string) bool {
	return util.StrContains(util.SuperRoles, subject) || r.Method == http.MethodGet
}

// EvalTopicAdminAPI evaluate tenant's topic administration permission
func EvalTopicAdminAPI(r *http.Request, subject string) bool {
	return util.StrContains(util.SuperRoles, subject) || r.Method == http.MethodGet
}

// TenantManager is the global objects to manage the Tenant REST API
var TenantManager TenantPolicyHandler

// Initialize initializes database
func Initialize() {
	if err := TenantManager.Setup(); err != nil {
		log.Fatal(err)
	}
}
