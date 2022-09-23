FROM golang:1.16.5-alpine3.12 as builder
WORKDIR /root/paas-prometheus-exporter
COPY . .
RUN go build

FROM alpine:3.16.2
COPY --from=builder /root/paas-prometheus-exporter /usr/local/bin
CMD paas-prometheus-exporter
