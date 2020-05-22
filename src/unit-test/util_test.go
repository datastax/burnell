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
