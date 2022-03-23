FROM registry.access.redhat.com/ubi8/go-toolset:1.16.12 AS builder
COPY . .
RUN go build -ldflags="-s -w" -o nvidia_gpu_exporter nvidia_gpu_exporter.go

FROM registry.access.redhat.com/ubi8/ubi-micro:8.4
COPY --from=builder /opt/app-root/src/nvidia_gpu_exporter /usr/bin/nvidia_gpu_exporter

EXPOSE      9445
ENTRYPOINT  ["/usr/bin/nvidia_gpu_exporter"]
