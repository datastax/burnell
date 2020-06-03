package logstream

import (
	"os"

	"github.com/kafkaesque-io/burnell/src/util"
)

// DefaultLogServerPort port
const DefaultLogServerPort = ":4040"

// FilePath is the default function log path
var FilePath = util.AssignString(os.Getenv("FunctionLogPathPrefix"), "/pulsar/logs/functions/")

// FunctionLogPath returns the absolute file name of function log.
func FunctionLogPath(tenant, namespace, function, instance string) string {
	return FilePath + tenant + "/" + namespace + "/" + function + "/" + function + "-" + instance + ".log"
}
