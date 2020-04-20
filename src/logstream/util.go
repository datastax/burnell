package logstream

import "strconv"

import (
	"os"

	"github.com/kafkaesque-io/burnell/src/util"
)

// DefaultLogServerPort port
const DefaultLogServerPort = ":4040"

// FilePath is the default function log path
var FilePath = util.AssignString(os.Getenv("FunctionLogPathPrefix"), "/pulsar/logs/functions/")

// FunctionLogPath returns the absolute file name of function log.
func FunctionLogPath(tenant, namespace, function string, instance int) string {
	return FilePath + tenant + "/" + namespace + "/" + function + "/" + function + "-" + strconv.Itoa(instance) + ".log"
}
