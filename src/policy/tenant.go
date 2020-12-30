package policy

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/apex/log"
	"github.com/datastax/burnell/src/util"
)

// maintains a list of tenants

var (
	// Tenants maintains an array of tenants, no need to use map since the list should be small
	Tenants = []string{}
)

func isTenant(tenant string) bool {
	for _, v := range Tenants {
		if tenant == v {
			return true
		}
	}
	return false
}

// IsTenant verifies if the tenant exists in the cluster
func IsTenant(tenant string) bool {
	if exists := isTenant(tenant); exists {
		return true
	}

	if err := updateTenants(); err != nil {
		log.Errorf("failed to query admin/v2/tenants error: %v", err)
	}
	return isTenant(tenant)
}

func updateTenants() error {

	requestURL := util.SingleJoinSlash(util.Config.BrokerProxyURL, "admin/v2/tenants")
	log.Infof("request route %s ", requestURL)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	newRequest.Header.Add("X-Proxy", "burnell")
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)

	client := &http.Client{
		CheckRedirect: util.PreserveHeaderForRedirect,
	}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		log.Errorf("%v", err)
		return errors.New("proxy failure")
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, &Tenants)
}
