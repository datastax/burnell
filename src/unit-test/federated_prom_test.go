package tests

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"

	. "github.com/kafkaesque-io/burnell/src/metrics"
)

func TestFederatedPromProcess(t *testing.T) {
	dat, err := ioutil.ReadFile("./federated-prom.dat")
	errNil(t, err)

	SetCache(string(dat))
	rc := FilterFederatedMetrics("victor")
	fmt.Println(rc)
	parts := strings.Split(rc, "\n")
	equals(t, 18, len(parts))
	typeDefPattern := fmt.Sprintf(`^# TYPE .*`)
	count := 0
	for _, v := range parts {
		matched, err := regexp.MatchString(typeDefPattern, v)
		if matched && err == nil {
			count++
		}
	}
	assert(t, 4 == count, "the number of type definition expected")

}
