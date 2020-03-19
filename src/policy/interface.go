package policy

import "time"

// PlanTier is the tenant plan
type PlanTier int

const (
	// FreeTier is free tier tenant policy
	FreeTier = iota
	// StarterTier is the starter tier
	StarterTier
	// ProductionTier is the production tier
	ProductionTier
	// StandardTier is the standard tier
	StandardTier
	// DedicatedTier is the decicated tier
	DedicatedTier
	// PrivateTier is the private tier
	PrivateTier
)

// TenantPolicy is the tenant policy
type TenantPolicy struct {
	Name            string
	Plan            PlanTier
	NumOfTopics     int
	NumOfNamespaces int
	MessageRetetion time.Duration
	NumOfProducers  int
	NumOfConsumers  int
	Functions       int
}

// TenantPolicyEvaluator evaluates the tenant management policy
type TenantPolicyEvaluator interface {
	Conn(hosts string) error
	GetPolicy(tenantName string) TenantPolicy
	Evaluate(tenantName string) error
}
