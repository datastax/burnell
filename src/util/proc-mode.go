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
