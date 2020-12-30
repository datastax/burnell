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
