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

package route

import (
	"net/http"
	"net/url"

	"github.com/apex/log"
	"github.com/datastax/burnell/src/util"
	"github.com/gorilla/websocket"
	wsproxy "github.com/koding/websocketproxy"
)

// WebsocketAuthProxyHandler is the websocket proxy
func WebsocketAuthProxyHandler(w http.ResponseWriter, r *http.Request) {
	proxyURLStr := util.AssignString(util.GetConfig().WebsocketURL, "ws://localhost:8000")
	if proxyURLStr == "" {
		log.Errorf("websocket proxy not configured")
		http.Error(w, "not configured", http.StatusNotImplemented)
		return
	}
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		log.Errorf("malformed proxy URL %s", proxyURLStr)
		http.Error(w, "consult with admin for malformed proxyURL", http.StatusInternalServerError)
		return
	}

	backend := func(r *http.Request) *url.URL {
		// Shallow copy
		u := proxyURL
		u.Fragment = r.URL.Fragment
		u.Path = r.URL.Path
		u.RawQuery = r.URL.RawQuery
		return u
	}
	director := func(incoming *http.Request, out http.Header) {
		u, _ := url.Parse(incoming.URL.String())
		params := u.Query()
		if tokenStr, ok := params["token"]; ok {
			out.Set("Authorization", "Bearer "+tokenStr[0])
		}
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	proxy := &wsproxy.WebsocketProxy{
		Backend:  backend,
		Director: director,
		Upgrader: &upgrader,
	}
	proxy.ServeHTTP(w, r)
}
