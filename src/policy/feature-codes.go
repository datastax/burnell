package policy

import (
	"fmt"
	"regexp"
	"strings"
)

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

///// internal implementation
//

// Feature is a struct to use and reinterpret FeatureCode
type Feature struct {
	PossibleCodes map[string]bool // impl Set
	FeatureCode
}

// FeatureCodeMap is a global map for feature code verification
var FeatureCodeMap map[string]Feature

// BuildFeatureCodeMap builds and verifies a map for feature code
func BuildFeatureCodeMap() error {
	FeatureCodeMap = make(map[string]Feature)
	for _, v := range KafkaesqueFeatureCodes {
		codes := make(map[string]bool)
		if name, ok := ValidateFeatureCode(v.Name); ok {
			codes[name] = true
		} else {
			return fmt.Errorf("invalid feature code name %s", v.Name)
		}
		aliases := strings.Split(v.Alias, ",")
		for _, alias := range aliases {
			if name, ok := ValidateFeatureCode(strings.TrimSpace(alias)); ok {
				codes[name] = true
			} else {
				return fmt.Errorf("invalid feature code alias %s", name)
			}
		}
		FeatureCodeMap[v.Name] = Feature{
			PossibleCodes: codes,
			FeatureCode:   v,
		}
	}
	return nil
}

// ValidateFeatureCode validate feature code conform the code regex naming convention.
func ValidateFeatureCode(name string) (string, bool) {
	p := strings.TrimSpace(strings.ToLower(name))
	r := regexp.MustCompile(`^[a-z0-9-]+$`)
	return p, r.MatchString(p)
}
