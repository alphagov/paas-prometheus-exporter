# paas-prometheus-exporter

This application consumes application and service metrics from Cloud Foundry. All the metrics are exposed on a  `/metrics` endpoint for a Prometheus server to scrape.

The application will get metrics for all apps and services that the user has access to.

To use paas-prometheus-exporter with GOV.UK PaaS, see [Use the PaaS Prometheus exporter app](https://docs.cloud.service.gov.uk/monitoring_apps.html#use-the-paas-prometheus-exporter-app) in the documentation.

If you use StatsD rather than Prometheus see [alphagov/paas-metric-exporter](https://github.com/alphagov/paas-metric-exporter).

## How metrics are collected

The applications and services are automatically discovered using the Cloud Foundry API every `update-frequency` seconds. This means it might take a little time while a new application or service's metrics are exposed.

If an application is stopped or deleted, or a service it deleted, the relevant metrics will be removed. (Only when the next app/service list refresh happens)

When the metrics collection for an app/service encounters an error it will be restarted, but the metrics might be missing for one update cycle. In this case you should see an error logged.

### Application metrics

For every application we create a persistent connection to a Doppler endpoint which automatically sends any new application event or metric. When a Prometheus scrape occurs the latest metric values are exposed.

### Service metrics

For every service instance we poll all metrics for the last 15 minutes from Log Cache every `scrape-interval` seconds. Only the newest values are used for any metrics. When a Prometheus scrape occurs we expose the state of the metrics from the last refresh, but we also add the original timestamps.

## Available application metrics

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

## Available service metrics

The service metrics are different service-by-service, please check the `/metrics` endpoint to see what metrics are exposed exactly.

## Getting Started

Refer to the [PaaS Technical Documentation](https://docs.cloud.service.gov.uk/monitoring_apps.html#metrics) for instructions on how to set up the metrics exporter app. Information on the configuration options are in the following table.

### Mandatory options

|Configuration Option|Application Flag|Environment Variable|Notes|
|:---|:---|:---|:---|
|API endpoint|api-endpoint|API_ENDPOINT||
|Username|username|USERNAME||
|Password|password|PASSWORD||

### Additional options

|Configuration Option|Application Flag|Environment Variable|Default value|Notes|
|:---|:---|:---|:---|:---|
|Bind port|prometheus-bind-port|PORT|8080|The port that the application will bind to.|
|Update frequency|update-frequency|UPDATE_FREQUENCY|300|The time in seconds, that takes between each apps update call|
|Scrape interval|scrape-interval|SCRAPE_INTERVAL|60|Scrape interval in seconds. Set this to the same value as the Prometheus scrape interval. The service metrics will be refreshed using the same interval|
|Log-cache endpoint|logcache-endpoint|LOGCACHE_ENDPOINT|`https://log-cache.<PaaS system domain>`|Usually it's unnecessary to override this|
|Basic-auth username|auth-username|AUTH_USERNAME|Apply basic auth protection to the /metrics endpoint|Leave this field blank to disable basic auth
|Basic-auth password|auth-password|AUTH_PASSWORD||

## Development

With each update of the PaaS Prometheus Exporter you should update the version number located in `main.go` file at the top of the `var` block.

## Testing

To run the test suite, then run `make test` from the root of this repository.

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

## Acknowledgements

The application is partly based on [`pivotal-cf/graphite-nozzle`](https://github.com/pivotal-cf/graphite-nozzle).
