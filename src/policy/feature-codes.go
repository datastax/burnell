package policy

// FeatureCode is a struct to define feature code and description
// Both name and alias will be used to match any possible spelling of the feature name.
// All feature name or alias will be switched to lowercase, so it is not case sensitive.
// It should be alphanumeric and -
type FeatureCode struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Alias       string `json:"alias"`
}

// KafkaesqueFeatureCodes is a list of features offered by Kafkaesque
var KafkaesqueFeatureCodes = []FeatureCode{
	FeatureCode{
		Name:        BrokerMetrics,
		Description: "exposes tenant broker metrics",
		Alias:       "broker-metrics,brokerMetrics",
	},
	FeatureCode{
		Name:        InfiniteMessageRetention,
		Description: "message infinite retention",
		Alias:       "infiniteMessageRetention,imr",
	},
	FeatureCode{
		Name:        "cluster-usage-tracking",
		Description: "tracks cluster usage by hours",
		Alias:       "cut,clusterUsageTracking",
	},
}
