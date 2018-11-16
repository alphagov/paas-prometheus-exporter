# paas-prometheus-exporter

This application consumes container metrics off the Cloud Foundry Doppler daemon, processes them based on the provided metrics template, and then sends them to a StatsD endpoint.

The application will get metrics for all apps that the user has access to.

The application is based on [`pivotal-cf/graphite-nozzle`](https://github.com/pivotal-cf/graphite-nozzle).

## Available metrics

The following metrics will be exported for every application instance.

|Name|Type|Description|
|:---|:---|:---|
|`cpu`|gauge|CPU utilisation|
|`diskBytes`|gauge|Disk usage in bytes|
|`diskUtilization`|gauge|Disk utilisation in percent (0-100)|
|`memoryBytes`|gauge|Memory usage in bytes|
|`memoryUtilization`|gauge|Memory utilisation in percent (0-100)|
|`crash`|counter|Increased by one if the application crashed for any reason|
|`requests.1xx`|counter|Number of requests processed with 1xx status|
|`requests.2xx`|counter|Number of requests processed with 2xx status|
|`requests.3xx`|counter|Number of requests processed with 3xx status|
|`requests.4xx`|counter|Number of requests processed with 4xx status|
|`requests.5xx`|counter|Number of requests processed with 5xx status|
|`requests.other`|counter|Number of requests processed with unknown status|
|`responseTime.1xx`|timer|Timing of processed requests with 1xx status|
|`responseTime.2xx`|timer|Timing of processed requests with 2xx status|
|`responseTime.3xx`|timer|Timing of processed requests with 3xx status|
|`responseTime.4xx`|timer|Timing of processed requests with 4xx status|
|`responseTime.5xx`|timer|Timing of processed requests with 5xx status|
|`responseTime.other`|timer|Timing of processed requests with unknown status|

## Getting Started

Refer to the [PaaS Technical Documentation](https://docs.cloud.service.gov.uk/monitoring_apps.html#metrics) for instructions on how to set up the metrics exporter app. Information on the configuration options are in the following table.

|Configuration Option|Application Flag|Environment Variable|Notes|
|:---|:---|:---|:---|
|API endpoint|api-endpoint|API_ENDPOINT||
|Username|username|USERNAME||
|Password|password|PASSWORD||
|Update frequency|update-frequency|UPDATE_FREQUENCY|The time in seconds, that takes between each apps update call|
|Prometheus Bind Port|prometheus-bind-port|PORT|The port that the prometheus server binds to. Default is 8080|

## Metric Whitelist

By default all the above metrics will be emitted.

You may restrict these, by composing a list of comma separated metric name
prefixes. For instance, in order to limit metrics to CPU, Disk usage, Response Times, and Requests metrics:

```sh
export METRIC_WHITELIST="cpu,diskBytes,responseTime,requests"
go run main.go
```

## Testing

To run the test suite, first make sure you have ginkgo and gomega installed:

```
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega
```

Then run `make test` from the root of this repository.

### Regenerating mocks

We generate some test mocks using counterfeiter. The mocks need to be regenerated if the mocked interfaces are changed.

To install counterfeiter please run first:
```
go get github.com/maxbrunsfeld/counterfeiter
```

To generate the mocks:
```
make generate
```
