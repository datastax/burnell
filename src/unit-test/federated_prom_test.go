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

package tests

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"

	. "github.com/datastax/burnell/src/metrics"
)

func TestFederatedPromProcess(t *testing.T) {
	dat, err := ioutil.ReadFile("./tenantusage.dat")
	errNil(t, err)

	SetCache("victor", dat)
	rc := FilterFederatedMetrics(dat, "victor")
	parts := strings.Split(rc, "\n")
	equals(t, 1, len(parts))
	typeDefPattern := fmt.Sprintf(`^# TYPE .*`)
	count := 0
	for _, v := range parts {
		matched, err := regexp.MatchString(typeDefPattern, v)
		if matched && err == nil {
			count++
		}
	}
	assert(t, 0 == count, "the number of type definition expected")

}

func TestTenantUsage(t *testing.T) {
	dat, err := ioutil.ReadFile("./tenantusage.dat")
	errNil(t, err)

	SetCache(SuperRole, dat)
	err = InitUsageDbTable()
	errNil(t, err)

	BuildTenantUsage()
	found := false
	usages, err := GetTenantsUsage()
	errNil(t, err)
	for _, v := range usages {
		if v.Name == "ming-luo" {
			fmt.Println(v)
			found = true
			equals(t, uint64(2681610), v.TotalBytesIn)
			equals(t, uint64(11360), v.TotalMessagesIn)
			equals(t, uint64(0), v.TotalBytesOut)
			equals(t, uint64(0), v.TotalMessagesOut)
			equals(t, uint64(6), v.MsgInBacklog)
		}
	}
	assert(t, found, "tenant matched")

	// test twice to ensure that cache has been completely overwritten
	BuildTenantUsage()
	// test twice to ensure that cache has been completely overwritten
	BuildTenantUsage()
	usages, err = GetTenantsUsage()
	errNil(t, err)
	for _, v := range usages {
		if v.Name == "ming-luo" {
			found = true
			equals(t, uint64(2681610), v.TotalBytesIn)
			equals(t, uint64(11360), v.TotalMessagesIn)
			equals(t, uint64(0), v.TotalBytesOut)
			equals(t, uint64(0), v.TotalMessagesOut)
			equals(t, uint64(6), v.MsgInBacklog)
		}
	}
	assert(t, found, "tenant matched")
}

func TestTenantNamespaceUsage(t *testing.T) {
	dat, err := ioutil.ReadFile("./tenantusage.dat")
	// dat, err := ioutil.ReadFile("./useast2-aws.dat")
	errNil(t, err)

	SetCache(SuperRole, dat)
	err = InitUsageDbTable()
	errNil(t, err)

	BuildTenantUsage()
	found := false
	usages, err := GetTenantNamespacesUsage("ming-luo")
	errNil(t, err)

	equals(t, 2, len(usages))

	for _, v := range usages {
		if v.Name == "ming-luo/namespace2" {
			fmt.Println(v)
			found = true
			equals(t, uint64(1084716), v.TotalBytesIn)
			equals(t, uint64(6594), v.TotalMessagesIn)
			equals(t, uint64(0), v.TotalBytesOut)
			equals(t, uint64(0), v.TotalMessagesOut)
			equals(t, uint64(0), v.MsgInBacklog)
		}
	}
	assert(t, found, "tenant matched")

	// test twice to ensure that cache has been completely overwritten
	BuildTenantUsage()
	// test twice to ensure that cache has been completely overwritten
	BuildTenantUsage()
	usages, err = GetTenantNamespacesUsage("ming-luo")
	errNil(t, err)

	for _, v := range usages {
		if v.Name == "ming-luo/namespace2" {
			found = true
			equals(t, uint64(1084716), v.TotalBytesIn)
			equals(t, uint64(6594), v.TotalMessagesIn)
			equals(t, uint64(0), v.TotalBytesOut)
			equals(t, uint64(0), v.TotalMessagesOut)
			equals(t, uint64(0), v.MsgInBacklog)
		}
	}
	assert(t, found, "tenant matched")
}
