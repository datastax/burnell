# Burnell

Burnell is a Pulsar proxy. It offers the following features.

- [x] Generate and validate Pulsar JWT that is compatible with Pulsar Java based Authen and Author
- [x] Authentication and authorization based on Pulsar JWT
- [x] Proxy the entire set of [Pulsar Admin REST API](https://pulsar.apache.org/admin-rest-api/)
- [ ] Tenant based authorization over Pulsar Admin REST API
- [x] Expose tenant Prometheus metrics on the brokers
- [ ] Interface to query function logs
- [ ] Proxy Pulsar TCP protocol

## Rest API

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
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:39:09.315634076-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":48,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":1,"brokerMetrics":2},"audit":"initial creation,"}
```
#### Update a tenant with a plan
Update can upgrade or downgrade plan or specifiy individual plan attributes and feature code.

`policy.messageHourRetention` (integer) is the data retention period. `policy.messageRetention` is reserved internally for Golang retention in nano-seconds.

```
-X POST
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
--data '{"PlanType": "free", "policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageRetention":172800000000000,"numofProducers":3:w
,"numOfConsumers":5,"functions":3,"brokerMetrics":1}}'
```
```
$ curl -v -X POST -H "Authorization: Bearer $MY_TOKEN" -d '{"planType": "free", "org": "", "users": "", policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":120,"numofProducers":3,"numOfConsumers":5,"functions":5,"brokerMetrics":1},"audit":"enable prometheus metrics"}' "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:44:40.494262281-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":120,"messageRetention":432000000000000,"numofProducers":3,"numOfConsumers":5,"functions":5,"brokerMetrics":1},"audit":"initial creation,,enable prometheus metrics"}
```
#### Get a tenant

```
-X GET
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
```
Example:
```
$ curl -H "Authorization: Bearer $MY_TOKEN" "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:39:09.315634076-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":48,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":1,"brokerMetrics":2},"audit":"initial creation,"}
```
#### DELETE a tenant with a plan 

```
-X DELETE
-H "Authorization: Bearer $SUPERROLE_TOKEN" \
```
Example:
```
$ curl -X DELETE -H "Authorization: Bearer $MY_TOKEN" "http://localhost:8964/k/tenant/ming-luo"
{"name":"ming-luo","tenantStatus":1,"org":"","users":"","planType":"free","updatedAt":"2020-04-17T13:39:09.315634076-04:00","policy":{"name":"free","numOfTopics":5,"numOfNamespaces":1,"messageHourRetention":48,"messageRetention":172800000000000,"numofProducers":3,"numOfConsumers":5,"functions":1,"brokerMetrics":2},"audit":"initial creation,"}
```

### Docker

```
docker build -t burnell .
```

docker tag kafkaesqueio/burnell:0.1.5