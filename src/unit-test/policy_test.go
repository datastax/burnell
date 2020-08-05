package tests

import (
	"fmt"
	"testing"

	. "github.com/kafkaesque-io/burnell/src/policy"
	"github.com/kafkaesque-io/burnell/src/util"
)

func TestFeatureCodes(t *testing.T) {
	_, ok := ValidateFeatureCode("feature code")
	assert(t, !ok, "space in feature code is not allowed")

	name, ok := ValidateFeatureCode("Feature-code")
	assert(t, name == "feature-code", "verify feature code name without uppercase")
	assert(t, ok, "a valid feature code")

	name, ok = ValidateFeatureCode(" FEATURECODE")
	assert(t, name == "featurecode", "verify feature code name without uppercase")
	assert(t, ok, "a valid feature code")

	_, ok = ValidateFeatureCode("feature_code")
	assert(t, !ok, "underscore is not allowed in feature code")

	name, ok = ValidateFeatureCode("feature-code123")
	assert(t, name == "feature-code123", "verify feature code name with alphanumeric and dash")
	assert(t, ok, "a valid feature code")

	assert(t, IsFeatureSupported(BrokerMetrics, FeatureAllEnabled), "test broker metrics against all enabled feature sets")
	assert(t, !IsFeatureSupported(BrokerMetrics, "broker-metric"), "test broker metrics against an invalid feature sets")
	assert(t, !IsFeatureSupported(BrokerMetrics, "infinite-message-retention,broker-metric,new-feature"), "test broker metrics against a list of invalid feature sets")
	assert(t, IsFeatureSupported(BrokerMetrics, "infinite-message-retention,broker-metrics,new-feature"), "test broker metrics against a list of valid feature sets")
	assert(t, !IsFeatureSupported(BrokerMetrics, ""), "test broker metrics against empty feature codes")
}

func TestFeatureCodeMap(t *testing.T) {
	err := BuildFeatureCodeMap()
	errNil(t, err)
	assert(t, len(FeatureCodeMap) == len(KafkaesqueFeatureCodes), "featureCodeMap matches the size of KafkaesqueFeatureCodes")
}

func TestIsPartitionTopic(t *testing.T) {
	name, is := IsPartitionTopic("persistent://ming-luo/local-useast1-gcp/partition-topic2-partition-0")
	fmt.Printf("name is %s", name)
	assert(t, name == "persistent://ming-luo/local-useast1-gcp/partition-topic2", "")
	assert(t, is, "")

	name, is = IsPartitionTopic("persistent://ming-luo/local-useast1-gcp/partitioned-topic2-partition-9")
	fmt.Printf("name is %s", name)
	assert(t, name == "persistent://ming-luo/local-useast1-gcp/partitioned-topic2", "")
	assert(t, is, "")

	name, is = IsPartitionTopic("persistent://ming-luo/local-useast1-gcp/partition-topic2-partition-19")
	assert(t, name == "persistent://ming-luo/local-useast1-gcp/partition-topic2", "")
	assert(t, is, "")

	_, is = IsPartitionTopic("persistent://ming-luo/local-useast1-gcp/partition-topic2-partitio-19")
	assert(t, !is, "")
	_, is = IsPartitionTopic("persistent://ming-luo/local-useast1-gcp/partition-topic2-partition-1o9")
	assert(t, !is, "")

	assert(t, util.IsPersistentTopic("persistent://ming-luo/local-useast1-gcp/partition-topic2-partition-1o9"), "")
	assert(t, !util.IsPersistentTopic("non-persistent://ming-luo/local-useast1-gcp/partition-topic2"), "")
}
