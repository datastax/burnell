package logstream

import "strconv"

// LogServerPort port
const LogServerPort = ":4040"

// FilePath is the default function log path
const FilePath = "/pulsar/logs/functions/"

// FunctionLogPath returns the absolute file name of function log.
func FunctionLogPath(tenant, namespace, function string, instance int) string {
	return FilePath + tenant + "/" + namespace + "/" + function + "/" + function + "-" + strconv.Itoa(instance) + ".log"
}
