package policy

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// PlanTier is the tenant plan
type PlanTier int

// TenantPolicyMap has tenant name as key, tenant policy as value
var TenantPolicyMap = cache.New(15*time.Minute, 3*time.Hour)

const (
	// FreeTier is free tier tenant policy
	FreeTier = iota
	// StarterTier is the starter tier
	StarterTier
	// ProductionTier is the production tier
	ProductionTier
	// DedicatedTier is the decicated tier
	DedicatedTier
	// PrivateTier is the private tier
	PrivateTier
)

// PlanPolicy is the tenant policy
type PlanPolicy struct {
	Name            string
	Plan            PlanTier
	NumOfTopics     int
	NumOfNamespaces int
	MessageRetetion time.Duration
	NumOfProducers  int
	NumOfConsumers  int
	Functions       int
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
		Name:            "Free Plan",
		Plan:            FreeTier,
		NumOfTopics:     5,
		NumOfNamespaces: 1,
		MessageRetetion: 2 * 24 * time.Hour,
		NumOfProducers:  3,
		NumOfConsumers:  5,
		Functions:       1,
	},
	StarterPlan: PlanPolicy{
		Name:            "Starter Plan",
		Plan:            StarterTier,
		NumOfTopics:     20,
		NumOfNamespaces: 2,
		MessageRetetion: 7 * 24 * time.Hour,
		NumOfProducers:  30,
		NumOfConsumers:  50,
		Functions:       10,
	},
	ProductionPlan: PlanPolicy{
		Name:            "Production Plan",
		Plan:            ProductionTier,
		NumOfTopics:     100,
		NumOfNamespaces: 6,
		MessageRetetion: 14 * 24 * time.Hour,
		NumOfProducers:  60,
		NumOfConsumers:  100,
		Functions:       20,
	},
	DedicatedPlan: PlanPolicy{
		Name:            "Dedicated Plan",
		Plan:            DedicatedTier,
		NumOfTopics:     1000,
		NumOfNamespaces: 500,
		MessageRetetion: 21 * 24 * time.Hour,
		NumOfProducers:  300,
		NumOfConsumers:  500,
		Functions:       30,
	},
	PrivatePlan: PlanPolicy{
		Name:            "Private Plan",
		Plan:            PrivateTier,
		NumOfTopics:     5000,
		NumOfNamespaces: 1000,
		MessageRetetion: 28 * 24 * time.Hour,
		NumOfProducers:  -1,
		NumOfConsumers:  -1,
		Functions:       -1,
	},
}

func getPlanPolicy(plan PlanTier) *PlanPolicy {
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
		return &TenantPlanPolicies.FreePlan
	}
}

// UpdateCache updates the tenant policy map
func UpdateCache(tenant string, plan PlanPolicy) {
	TenantPolicyMap.Add(tenant, plan, cache.DefaultExpiration)
}
