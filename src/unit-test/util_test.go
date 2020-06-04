package tests

import (
	"os"
	"testing"

	. "github.com/kafkaesque-io/burnell/src/util"
)

func TestGetEnvInt(t *testing.T) {
	assert(t, GetEnvInt("Bogus", 546) == 546, "")

	os.Setenv("Bogus2", "-90")
	assert(t, GetEnvInt("Bogus2", 546) == -90, "")
	os.Setenv("Bogus2", "-90o")
	assert(t, GetEnvInt("Bogus2", 546) == 546, "")
}

func TestConditionAssign(t *testing.T) {
	assert(t, ConditionAssign(true, "testme", "test2") == "testme", "")
	assert(t, ConditionAssign(false, "testme", "test2") == "test2", "")
}

func TestPartitionTopicPrefix(t *testing.T) {
	name, isPartitionTopic := ParsePartitionTopicName("partition-persistent://tenant/ns/topic")
	assert(t, name == "persistent://tenant/ns/topic", "")
	assert(t, isPartitionTopic, "")

	name, isPartitionTopic = ParsePartitionTopicName("partition-persistent://tenant/ns/topic-partition")
	assert(t, name == "persistent://tenant/ns/topic-partition", "")
	assert(t, isPartitionTopic, "")

	name, isPartitionTopic = ParsePartitionTopicName("persistent://tenant/ns/topic-partition")
	assert(t, name == "persistent://tenant/ns/topic-partition", "")
	assert(t, !isPartitionTopic, "")

	name, isPartitionTopic = ParsePartitionTopicName("partition-non-persistent://tenant/ns/topic-partition")
	assert(t, name == "non-persistent://tenant/ns/topic-partition", "")
	assert(t, isPartitionTopic, "")
}

func TestExtractPartsFromTopicFn(t *testing.T) {
	tn, ns, topic, err := ExtractPartsFromTopicFn("persistent://tenant-ab/namespace2/topic789")
	errNil(t, err)
	assert(t, tn == "tenant-ab", "valid tenant")
	assert(t, ns == "namespace2", "valid namespace")
	assert(t, topic == "topic789", "valid topic")

	tn, ns, topic, err = ExtractPartsFromTopicFn("persisten://tenant-ab/namespace2/topic789")
	assert(t, err != nil, "")

	tn, ns, topic, err = ExtractPartsFromTopicFn("persistent://tenant-ab/namespace2/topic789/anotherpath")
	assert(t, err != nil, "")

	tn, ns, topic, err = ExtractPartsFromTopicFn("persistent://tenant-ab/namespace2")
	assert(t, err != nil, "")

	tn, ns, topic, err = ExtractPartsFromTopicFn("persistent:/tenant-ab/namespace2/topic789")
	assert(t, err != nil, "")

}

func TestBytesToMB(t *testing.T) {
	assert(t, 1 == BytesToMegaBytesFloor(1000), "test 1000 bytes")
	assert(t, 1 == BytesToMegaBytesFloor(1000*900), "test 1000*900 bytes")
	assert(t, 1 == BytesToMegaBytesFloor(1000*1500), "test 1500 * 1000 bytes")
	assert(t, 2 == BytesToMegaBytesFloor(1000*2000), "test  2 megabytes")
	assert(t, 40 == BytesToMegaBytesFloor(40479809), "test  megabytes")
}
