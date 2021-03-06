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

	"github.com/apex/log"
	"github.com/datastax/burnell/src/util"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealerRouter creates http routes for healer running mode
func HealerRouter() *mux.Router {
	log.Warnf("set up healer routes")

	router := mux.NewRouter().StrictSlash(true)

	router.Path("/liveness").Methods(http.MethodGet).Name("liveness").Handler(NoAuth(Logger(http.HandlerFunc(StatusPage), "liveness")))
	router.Path("/metrics").Methods(http.MethodGet).Name("metrics").Handler(NoAuth(promhttp.Handler()))
	return router
}

// NewRouter - create new router for HTTP routing
func NewRouter() *mux.Router {
	log.Warnf("set up proxy routes")

	router := mux.NewRouter().StrictSlash(true)

	// Order of routes definition matters

	router.Path("/liveness").Methods(http.MethodGet).Name("liveness").Handler(NoAuth(Logger(http.HandlerFunc(StatusPage), "liveness")))
	router.Path("/subject/{sub}").Methods(http.MethodGet).Name("token server").Handler(SuperRoleRequired(Logger(http.HandlerFunc(TokenSubjectHandler), "token server")))
	router.PathPrefix("/ws/").Name("websocket proxy proxy").
		Handler(http.HandlerFunc(WebsocketAuthProxyHandler))
	router.Path("/metrics").Methods(http.MethodGet).Name("metrics").Handler(NoAuth(promhttp.Handler()))
	router.Path("/tenantsusage").Methods(http.MethodGet).Name("tenants usage").Handler(SuperRoleRequired(http.HandlerFunc(TenantUsageHandler)))
	router.Path("/namespacesusage/{tenant}").Methods(http.MethodGet).Name("tenant namespaces usage").Handler(AuthVerifyTenantJWT(http.HandlerFunc(TenantUsageHandler)))
	router.Path("/pulsarmetrics/{tenant}").Methods(http.MethodGet).Name("pulsar metrics").
		Handler(SuperRoleRequired(http.HandlerFunc(PulsarFederatedDebugPrometheusHandler)))
	router.Path("/pulsarmetrics").Methods(http.MethodGet).Name("pulsar metrics").
		Handler(AuthVerifyJWT(http.HandlerFunc(PulsarFederatedPrometheusHandler)))

	// Tenant policy management URL
	router.Path("/k/tenant/{tenant}").Methods(http.MethodGet).Name("kafkaesque tenant management GET").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(TenantManagementHandler)))
	router.Path("/k/tenant/{tenant}").Methods(http.MethodDelete, http.MethodPost).Name("kafkaesque tenant management").
		Handler(SuperRoleRequired(http.HandlerFunc(TenantManagementHandler)))

	if util.GetConfig().PulsarBeamTopic != "" {
		// Pulsar Beam topic and webhook management URL
		router.Path("/pulsarbeam/v2/topic").Methods(http.MethodGet).Name("Pulsar Beam Get a topic").
			Handler(AuthVerifyJWT(http.HandlerFunc(PulsarBeamGetTopicHandler)))
		router.Path("/pulsarbeam/v2/topic").Methods(http.MethodDelete).Name("Pulsar Beam Delete a topic").
			Handler(AuthVerifyJWT(http.HandlerFunc(PulsarBeamDeleteTopicHandler)))
		router.Path("/pulsarbeam/v2/topic/{topicKey}").Methods(http.MethodGet).Name("Pulsar Beam Get a topic").
			Handler(AuthVerifyJWT(http.HandlerFunc(PulsarBeamGetTopicHandler)))
		router.Path("/pulsarbeam/v2/topic/{topicKey}").Methods(http.MethodDelete).Name("Pulsar Beam Delete a topic").
			Handler(AuthVerifyJWT(http.HandlerFunc(PulsarBeamDeleteTopicHandler)))
		router.Path("/pulsarbeam/v2/topic").Methods(http.MethodPost).Name("Pulsar Beam Update a topic").
			Handler(AuthVerifyJWT(http.HandlerFunc(PulsarBeamUpdateTopicHandler)))
	}

	// Collect tenant topics statistics in one call
	router.Path("/stats/topics/{tenant}").Methods(http.MethodGet).Name("tenant topic stats").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(TenantTopicStatsHandler)))

	// Retrieve function logs, instance is optional and default to 0
	router.Path("/function-logs/{tenant}/{namespace}/{function}").Methods(http.MethodGet).Name("function-logs").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(FunctionLogsHandler)))
	router.Path("/function-logs/{tenant}/{namespace}/{function}/{instance}").Methods(http.MethodGet).Name("function-logs").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(FunctionLogsHandler)))
	router.Path("/function-status/{tenant}/{namespace}/{function}").Methods(http.MethodGet).Name("function-logs-status").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(FunctionStatusHandler)))

	// aggregated topics under namespaces
	router.Path("/admin/v2/topics/{tenant}").Methods(http.MethodGet).Name("topics-grouped-by-namespaces").
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(GroupTopicsByNamespaceHandler)))

	// Pulsar Admin REST API proxy
	//
	// /bookies/
	router.PathPrefix("/admin/v2/bookies").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))

	// /broker-stats
	router.PathPrefix("/admin/v2/broker-stats").Methods(http.MethodGet).
		Handler(SuperRoleRequired(http.HandlerFunc(BrokerAggregatorHandler)))
	// Exception is broker-resource-availability/{tenant}/{namespace}
	// since "org.apache.pulsar.broker.loadbalance.impl.ModularLoadManagerWrapper does not support this operation"
	// we would not support this for now

	//
	// /brokers
	//
	router.PathPrefix("/admin/v2/brokers").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))

	//
	// /clusters
	//
	router.PathPrefix("/admin/v2/clusters").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// /namespaces
	// list of routes in the look up order from more restricted to relaxed including JWT role authorization
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/maxConsumersPerSubscription").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/maxConsumersPerTopic").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/maxProducersPerTopic").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/maxUnackedMessagesPerSubscription").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/messageTTL").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/offloadDeletionLagMs").Methods(http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/offloadPolicies").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/offloadThreshold").Methods(http.MethodPut).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/schemaAutoUpdateCompatibilityStrategy").Methods(http.MethodPut).
		Handler(SuperRoleRequired(http.HandlerFunc(NamespacePolicyProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/schemaCompatibilityStrategy").Methods(http.MethodPut).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(NamespacePolicyProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/schemaValidationEnforced").Methods(http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(NamespacePolicyProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/deduplication").Methods(http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectBrokerProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/permissions/{role}").Methods(http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/persistence").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/replication").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/replicatorDispatchRate").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/retention").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/subscribeRate").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/subscriptionAuthMode").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/subscriptionDispatchRate").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/unload").Methods(http.MethodPut).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/{bundle}").Methods(http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/{bundle}/split").Methods(http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/{bundle}/unload").Methods(http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/{bundle}/clearBacklog").Methods(http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/{bundle}/clearBacklog/{subscription}").Methods(http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/{bundle}/unsubscribe/{subscription}").Methods(http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectBrokerProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/autoSubscriptionCreation").Methods(http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/autoTopicCreation").Methods(http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/backlogQuota").Methods(http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/backlogQuotaMap").Methods(http.MethodGet).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	// this includes clearBacklog/{subscription}
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/clearBacklog").Methods(http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectBrokerProxyHandler)))
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/antiAffinity").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/compactionThreshold").Methods(http.MethodGet, http.MethodPut).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}/delayedDelivery").Methods(http.MethodGet, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	// including admin/v2/namespaces/{tenant}/{namespace}/dispatchRate,
	// including admin/v2/namespaces/{tenant}/{namespace}/isAllowAutoUpdateSchema
	router.PathPrefix("/admin/v2/namespaces/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(NamespaceLimitEnforceProxyHandler)))

	router.PathPrefix("/admin/v2/namespaces/{tenant}").Methods(http.MethodGet, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	// 2. routes require superroles access
	// including admin/v2/namespaces/{cluster}/antiAffinity/{group}
	// admin/v2/namespaces/{property}/{namespace}/persistence/bookieAffinity
	//
	router.PathPrefix("/admin/v2/namespaces").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// persistent topic
	//
	router.PathPrefix("/admin/v2/persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(TopicProxyHandler)))

	// /admin/v2/persistent/{tenant}/{namespace}/partitioned

	// non-persistent topic
	router.PathPrefix("/admin/v2/non-persistent/{tenant}/{namespace}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(TopicProxyHandler)))

	//
	// /resource-quotas
	//
	router.PathPrefix("/admin/v2/resource-quotas").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// /schemas
	//
	router.PathPrefix("/admin/v2/schemas/{tenant}/{namespace}/{topic}/compatibility").Methods(http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/schemas/{tenant}/{namespace}/{topic}/schema").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/schemas/{tenant}/{namespace}/{topic}/schema/{version}").Methods(http.MethodGet).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/schemas/{tenant}/{namespace}/{topic}/schemas").Methods(http.MethodGet).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/schemas/{tenant}/{namespace}/{topic}/version").Methods(http.MethodPost).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
		// catch all routes
	router.PathPrefix("/admin/v2/schemas/{tenant}").Methods(http.MethodGet, http.MethodPost, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))

	//
	// /tenants
	//
	router.PathPrefix("/admin/v2/tenants/{tenant}").Methods(http.MethodGet).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(CachedProxyHandler)))
	router.PathPrefix("/admin/v2/tenants").Methods(http.MethodGet).
		Handler(AuthVerifyJWT(http.HandlerFunc(RestrictedTenantsProxyHandler)))
	router.PathPrefix("/admin/v2/tenants").Methods(http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(SuperRoleRequired(http.HandlerFunc(CachedProxyHandler)))

	//
	// /functions including v2 for backward compatibility
	//
	// routes /admin/v3/functions/connectors is not supported by proxy 8443 either
	router.PathPrefix("/admin/v3/functions/{tenant}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectFunctionProxyHandler)))

	router.PathPrefix("/admin/v2/functions/{tenant}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectFunctionProxyHandler)))

	//
	// /sources
	//
	router.PathPrefix("/admin/v3/sources/builtinsources").Methods(http.MethodGet).
		Handler(AuthVerifyJWT(http.HandlerFunc(DirectFunctionProxyHandler)))

	router.PathPrefix("/admin/v3/sources/reloadBuiltInSources").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectFunctionProxyHandler)))

	router.PathPrefix("/admin/v3/sources/{tenant}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectFunctionProxyHandler)))

	//
	// /sinks
	//
	router.PathPrefix("/admin/v3/sinks/builtinsinks").Methods(http.MethodGet).
		Handler(AuthVerifyJWT(http.HandlerFunc(DirectFunctionProxyHandler)))

	router.PathPrefix("/admin/v3/sinks/reloadBuiltInSinks").Methods(http.MethodPost).
		Handler(SuperRoleRequired(http.HandlerFunc(DirectFunctionProxyHandler)))

	router.PathPrefix("/admin/v3/sinks/{tenant}").Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete).
		Handler(AuthVerifyTenantJWT(http.HandlerFunc(DirectFunctionProxyHandler)))

	// TODO rate limit can be added per route basis
	router.Use(LimitRate)

	router.Use(ResponseJSONContentType)

	log.Warnf("router added")
	return router
}
