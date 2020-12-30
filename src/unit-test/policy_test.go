 //
 //  Copyright (c) 2021 Datastax, Inc.
 //  
 //  Licensed to the Apache Software Foundation (ASF) under one
 //  or more contributor license agreements.  See the NOTICE file
 //  distributed with this work for additional information
 //  regarding copyright ownership.  The ASF licenses this file
 //  to you under the Apache License, Version 2.0 (the
 //  "License"); you may not use this file except in compliance
 //  with the License.  You may obtain a copy of the License at
 //  
 //     http://www.apache.org/licenses/LICENSE-2.0
 //  
 //  Unless required by applicable law or agreed to in writing,
 //  software distributed under the License is distributed on an
 //  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 //  KIND, either express or implied.  See the License for the
 //  specific language governing permissions and limitations
 //  under the License.
 //

package tests

import (
	"fmt"
	"testing"

	. "github.com/datastax/burnell/src/policy"
	"github.com/datastax/burnell/src/util"
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
