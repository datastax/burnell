# Burnell

Burnell is a Pulsar proxy. It offers the following features.

- [x] Generate and validate Pulsar JWT that is compatible with Pulsar Java based Authen and Author
- [x] Authentication and authorization based on Pulsar JWT
- [x] Proxy the entire set of [Pulsar Admin REST API](https://pulsar.apache.org/admin-rest-api/)
- [x] Tenant based authorization over Pulsar Admin REST API
- [x] Expose tenant Prometheus metrics on the brokers
- [x] Interface to query function logs
- [x] Pulsar Beam webhook and topic management REST API
- [x] Tenants and tenant's namespace usage metering, total bytes and message in and out
- [x] Initializer mode to configure Pulsar Kubernete cluster TLS keys and JWT
- [x] Healer mode to repair Pulsar Kubernete cluster TLS keys and JWT
- [ ] Proxy Pulsar TCP protocol

## Process running mode
```
burnell -mode init
burnell -mode healer
burnell -mode proxy
```
The default process mode is `proxy`

## Rest API

### Generate user token
A super user role's JWT must be specified in the `Authorization` header as `Bearer` token in the `GET` method with this route

```
/subject/{user-subject}
```
Allowed characters for user subject are alphanumeric and hyphen.

Token signing method and expiry duration can be passed as query parameters. The default settings are RS256 and no expiry.
```
/subject/{user-subject}?exp=<duration>&alg=<signMethod>
```
The supported duration is d, y, and [ns, us, µs, ms, s, m, h defined by Go time package](https://golang.org/pkg/time/#ParseDuration)
The supported signing method is sepcified at [here]()

### Tenant function log retrieval
It is a rolling log retrieval from the function worker.

#### Function log retrieval endpoint and query parameters
By default, the endpoint will return the last few lines of the latest log.
```
/function-logs/{tenant}/{namespace}/{function-name}
/function-logs/{tenant}/{namespace}/{function-name}/{instance}
```
Default instance is 0 if not specified.

Example -
```
/function-logs/ming-luo/namespace2/for-monitor-function
```

To retrieve earlier logs or newer logs, use query parameters `backwardpos` or `forwardpos` that indicates the byte index in the log file. These values can be obtained from the response body of the last query, under these attributes `BackwardPosition` and `ForwardPosition`. We run an algorithm to return a few complete lines. It is important to use the exact positions in the response body to have those complete log lines. An arbitary position will result truncated logs.
```
/function-logs/{tenant}/{namespace}/{function-name}?backwardpos=45000
```
It is client's responsiblity to keep track of the log file traverse position in both backward and forward position. The number must be a positive number. 0 value of `backwardpos` or `forwardpos` resets the `backwardpos` to EOF of the log file that displays the last few lines.

Since the algorithm always returns a few complete logs, the payload size can vary. Usualy the size ranges from one or two kilobytes.
```
{
    "BackwardPosition": 47987,
    "ForwardPosition": 49427,
    "Logs": "[2020-03-30 12:31:57 +0000] [ERROR] log.py: Traceback (most recent call last):\n[2020-03-30 12:31:57 +0000] [ERROR] log.py:   File \"/pulsar/instances/python-instance/python_instance_main.py\", line 211, in <module>\n[2020-03-30 12:31:57 +0000] [ERROR] log.py: main()\n[2020-03-30 12:31:57 +0000] [ERROR] log.py:   File \"/pulsar/instances/python-instance/python_instance_main.py\", line 192, in main\n[2020-03-30 12:31:57 +0000] [ERROR] log.py: pyinstance.run()\n[2020-03-30 12:31:57 +0000] [ERROR] log.py:   File \"/pulsar/instances/python-instance/python_instance.py\", line 189, in run\n[2020-03-30 12:31:57 +0000] [ERROR] log.py: **consumer_args\n"
}
```

#### Function worker Id per function instances
To help troubleshoot function instance and its worker Id mapping, the `function-status` endpoint offers insights of such mapping and function status.
```
/function-status/{tenant}/{namespace}/{function-name}
```

### Tenant topics statistics collector

#### Topic stats endpoint
METHOD: GET
```
/stats/topics/{tenant}?limit=10&offset=0
```
A list of required topics can be specified in the request body. This feature is useful since this endpoint usually retreives topics from a local cache that has 5 seconds polling interval, the mandatory list will directly query these topics against the broker admin REST endpoint.
```
{"tenant":"ming-luo","sessionId":"reserverd for snapshot iteration","offset":1,"total":1,"data":{"persistent://ming-luo/namespace2/test-topic3":{"averageMsgSize":0,"backlogSize":0,"msgRateIn":0,"msgRateOut":0,"msgThroughputIn":0,"msgThroughputOut":0,"pendingAddEntriesCount":0,"producerCount":0,"publishers":[],"replication":{},"storageSize":0,"subscriptions":{"mysub":{"consumers":[],"msgBacklog":0,"msgRateExpired":0,"msgRateOut":0,"msgRateRedeliver":0,"msgThroughputOut":0,"numberOfEntriesSinceFirstNotAckedMessage":1,"totalNonContiguousDeletedMessagesRange":0,"type":"Exclusive"}}}}}
```

### Grouping topics under namespace per tenant
```
/admin/v2/topics/{tenant}
```

### Tenant and namespace level usage metering
#### Tenant usage metering endpoint
Returns all tenants' usage metering data including the number of messages and total bytes in and out, and backlog size
Superuser token is required
```
/tenantsusage
```
#### Tenant usage metering endpoint
Returns individual namespaces' usage metering data including the number of messages and total bytes in and out, and backlog size, under a tenant
Superuser token or tenant token is required
```
/namespacesusage/{tenant}
```

### Tenant CRUD

#### Resource endpoint
```
/k/tenant/{tenant}
```
`{tenant}`, tenant name, must be specified for all REST calls.

#### Support HTTP Method 
`http.MethodGet, http.MethodDelete, http.MethodPost`

#### Header

`-H "Authorization: Bearer $SUPERROLE_TOKEN"`
Superrole token is required.

#### Create a tenant with a plan 

```
-X POST
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
--data '{"PlanType": "free"}'
```
Example:
```
$ curl -v -X POST -H "Authorization: Bearer $MY_TOKEN" -d '{"planType": "free"}' "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:39:09.315634076-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":48,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":1,"featureCodes":""},"audit":"initial creation,"}
```
#### Update a tenant with a plan
Update can upgrade or downgrade plan or specifiy individual plan attributes and feature code.

`policy.messageHourRetention` (integer) is the data retention period. `policy.messageRetention` is reserved internally for Golang retention in nano-seconds.

```
-X POST
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
--data '{"PlanType": "free", "policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":3,"featureCodes":"broker-metrics"}}'
```
```
$ curl -v -X POST -H "Authorization: Bearer $MY_TOKEN" -d '{"planType": "free", "org": "", "users": "", policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":120,"numofProducers":3,"numOfConsumers":5,"functions":5,"featureCodes":1},"audit":"enable prometheus metrics"}' "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:44:40.494262281-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":120,"messageRetention":432000000000000,"numofProducers":3,"numOfConsumers":5,"functions":5,"featureCodes":"broker-metrics"},"audit":"initial creation,,enable prometheus metrics"}
```
#### Get a tenant

```
-X GET
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
```
Example:
```
$ curl -H "Authorization: Bearer $MY_TOKEN" "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:39:09.315634076-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":48,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":1,"featureCodes":""},"audit":"initial creation,"}
```
#### DELETE a tenant with a plan 

```
-X DELETE
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
```
Example:
```
$ curl -X DELETE -H "Authorization: Bearer $MY_TOKEN" "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:39:09.315634076-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":48,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":1,"featureCodes":""},"audit":"initial creation,"}
```

### Tenant based Prometheus Metrics
Expose `\pulsarmetrics` endpoint with Pulsar prometheus metrics pertaining to the tenant. The tenant is identified based on the Authorization token.

If a superuser token is supplied, all the federated prometheus metrics will be returned.

#### Scrape job requirement
Scrape config offers `honor_labels: true` to honor the existing labels. It is optional because `exported_` labels can be used to identify metrics. But, the scrape job should set `honor_timestamps: true` to retain the original timestamp. Here is the [detail scrape config description](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config)

### Pulsar Admin Rest API Proxy

#### Pulsar Admin REST API
[Pulsar Admin REST API](https://pulsar.apache.org/admin-rest-api/) is mapped with the exact same routes. The exceptions are `\broker-stats`.

`/admin/v2/broker-stats` response aggregates all brokers' statistics. It returns brokers in the order of broker name, therefore it individual broker json object similar to pagination.

```
-X GET
-H "Authorization: Bearer $SUPERROLE_TOKEN"
"https://<pulsar proxy server fqdn>:8964/admin/v2/broker-stats/metrics?limit=2&offset=0"
```

`limit=0` will return all brokers in a single call. 

Without limit and offset query parameters, it will return the very first broker in the order broker's alphanumeric order.

The new offset and total will be returned in the repsonse body.

```
{"total":1,"offset":1,"data":[{"broker":"10.244.1.221:8080","data":[{"...
```

### Docker

```
docker build -t burnell .
```

### Docker for logcollector

```
docker build -t burnell-logcollector -f ./dockerfiles/logserver/Dockerfile .
docker run --rm -it -p 4042:4042 -e "LogServerPort=:4042" --name burnell-logcollector burnell-logcollector:latest
```