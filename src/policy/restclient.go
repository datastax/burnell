package policy

import "net/url"

// RestClient is the client object for RestAPI
type RestClient struct {
	URL *url.URL
}

// Conn sets up the server string
func (r *RestClient) Conn(hosts string) error {
	var err error
	r.URL, err = url.ParseRequestURI(hosts)

	return err
}

// GetPlanPolicy gets the policy
func (r *RestClient) GetPlanPolicy(tenantName string) PlanPolicy {
	return PlanPolicy{}
}

// Evaluate gets the policy
func (r *RestClient) Evaluate(tenantName string) error {
	return nil
}
