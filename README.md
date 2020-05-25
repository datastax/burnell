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
- [ ] Proxy Pulsar TCP protocol

## Rest API

### Tenant function log retrieval
It is a rolling log retrieval from the function worker.

#### Function log retrieval endpoint and query parameters
By default, the endpoint will return the last few lines of the latest log.
```
/function-logs/{tenant}/{namespace}/{function-name}
```
Example -
```
/function-logs/ming-luo/namespace2/for-monitor-function
```

Query parameters allows retrieve previous logs or newer logs.
```
/function-logs/{tenant}/{namespace}/{function-name}?bytes=1000?backwardpos=45000
```
The response body returns logs in complete lines (EOL) up to the maximum bytes specified in the `bytes` query parameters.

`backwardpos` and `forwardpos` allows the log to be retrieved from either backward or forward position from where the log rolling initially started. The response body will specify the current backward and forward positions that can be used for the next calls query parameters.
```
{
    "BackwardPosition": 47987,
    "ForwardPosition": 49427,
    "Logs": "[2020-03-30 12:31:57 +0000] [ERROR] log.py: Traceback (most recent call last):\n[2020-03-30 12:31:57 +0000] [ERROR] log.py:   File \"/pulsar/instances/python-instance/python_instance_main.py\", line 211, in <module>\n[2020-03-30 12:31:57 +0000] [ERROR] log.py: main()\n[2020-03-30 12:31:57 +0000] [ERROR] log.py:   File \"/pulsar/instances/python-instance/python_instance_main.py\", line 192, in main\n[2020-03-30 12:31:57 +0000] [ERROR] log.py: pyinstance.run()\n[2020-03-30 12:31:57 +0000] [ERROR] log.py:   File \"/pulsar/instances/python-instance/python_instance.py\", line 189, in run\n[2020-03-30 12:31:57 +0000] [ERROR] log.py: **consumer_args\n"
}
```


### Tenant topics statistics collector

#### Topic stats endpoint
```
/stats/topics/{tenant}?limit=10&offset=0
```
```
{"tenant":"ming-luo","sessionId":"reserverd for snapshot iteration","offset":1,"total":1,"data":{"persistent://ming-luo/namespace2/test-topic3":{"averageMsgSize":0,"backlogSize":0,"msgRateIn":0,"msgRateOut":0,"msgThroughputIn":0,"msgThroughputOut":0,"pendingAddEntriesCount":0,"producerCount":0,"publishers":[],"replication":{},"storageSize":0,"subscriptions":{"mysub":{"consumers":[],"msgBacklog":0,"msgRateExpired":0,"msgRateOut":0,"msgRateRedeliver":0,"msgThroughputOut":0,"numberOfEntriesSinceFirstNotAckedMessage":1,"totalNonContiguousDeletedMessagesRange":0,"type":"Exclusive"}}}}}
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

docker tag kafkaesqueio/burnell:0.1.5

### Docker for logcollector

```
docker build -t burnell-logcollector -f ./dockerfiles/logserver/Dockerfile .
docker run --rm -it -p 4042:4042 -e "LogServerPort=:4042" --name burnell-logcollector burnell-logcollector:latest
```