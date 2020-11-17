package util

//it is control block to determine the main running mode

// Proxy is the proxy mode to Pulsar REST admin/v2, prometheus, and function log proxy
const Proxy = "proxy"

// Initializer is the initializer to configure Pulsar cluster such as privision TLS keys and tokens
const Initializer = "init"

// Healer repairs any misconfiguration in an already deployed cluster
const Healer = "healer"

// IsInitializer check if the broker is required
func IsInitializer(mode *string) bool {
	return *mode == Initializer
}

// IsHealer is the process mode healer
func IsHealer(mode *string) bool {
	return *mode == Healer
}
