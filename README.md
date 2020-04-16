# Burnell

Burnell is a Pulsar proxy. It offers the following features.

- [x] Generate and validate Pulsar JWT that is compatible with Pulsar Java based Authen and Author
- [x] Authentication and authorization based on Pulsar JWT
- [x] Proxy the entire set of [Pulsar Admin REST API](https://pulsar.apache.org/admin-rest-api/)
- [ ] Tenant based authorization over Pulsar Admin REST API
- [x] Expose tenant Prometheus metrics on the brokers
- [ ] Interface to query function logs
- [ ] Proxy Pulsar TCP protocol

### Docker

```
docker build -t burnell .
```

docker tag kafkaesqueio/burnell:0.1.5