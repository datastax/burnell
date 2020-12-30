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
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

// ZookeeperDriver is the zookeeper database connector
type ZookeeperDriver struct {
	zkDriver *zk.Conn
}

// Conn connects to zookeeper
func (z *ZookeeperDriver) Conn(hosts string) error {
	hostList := strings.Split(hosts, ",")
	// hostList := []string{"127.0.0.1"}

	var err error
	z.zkDriver, _, err = zk.Connect(hostList, 4*time.Second)
	if err != nil {
		return err
	}
	/*_, __, _, err := zkDriver.ChildrenW("/")
	if err != nil {
		return err
	}
	*/
	return nil
}

// GetPlanPolicy gets the policy
func (z *ZookeeperDriver) GetPlanPolicy(tenantName string) PlanPolicy {
	return PlanPolicy{}
}

// Evaluate gets the policy
func (z *ZookeeperDriver) Evaluate(tenantName string) error {
	return nil
}
