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
