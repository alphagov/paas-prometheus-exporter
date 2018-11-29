# paas-prometheus-exporter

This application consumes metrics off the Cloud Foundry Doppler daemon and exposes them on a `/metrics` endpoint for a Prometheus server to scrape.

The application will get metrics for all apps that the user has access to.

The application is based on [`pivotal-cf/graphite-nozzle`](https://github.com/pivotal-cf/graphite-nozzle).

## Available metrics

The following metrics will be exported for every application instance.

|Name|Type|Description|
|:---|:---|:---|
|`cpu`|gauge|CPU utilisation in percent (0-100)|
|`disk_bytes`|gauge|Disk usage in bytes|
|`disk_utilization`|gauge|Disk utilisation in percent (0-100)|
|`memory_bytes`|gauge|Memory usage in bytes|
|`memory_utilization`|gauge|Memory utilisation in percent (0-100)|
|`crash`|counter|Increased by one if the application crashed for any reason|
|`requests`|counter|Number of requests processed broken down by `status_range` label|
|`response_time`|histogram|Timing of processed requests broken down by `status_range` label|

## Getting Started

Refer to the [PaaS Technical Documentation](https://docs.cloud.service.gov.uk/monitoring_apps.html#metrics) for instructions on how to set up the metrics exporter app. Information on the configuration options are in the following table.

|Configuration Option|Application Flag|Environment Variable|Notes|
|:---|:---|:---|:---|
|API endpoint|api-endpoint|API_ENDPOINT||
|Username|username|USERNAME||
|Password|password|PASSWORD||
|Update frequency|update-frequency|UPDATE_FREQUENCY|The time in seconds, that takes between each apps update call|
|Prometheus Bind Port|prometheus-bind-port|PORT|The port that the prometheus server binds to. Default is 8080|

## Development

With each update of the PaaS Prometheus Exporter you should update the version number located in `main.go` file at the top of the `var` block.

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
