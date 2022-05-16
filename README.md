![lint](https://github.com/smgglrs/nvidia-gpu-exporter/actions/workflows/lint.yaml/badge.svg)

# NVIDIA GPU Prometheus Exporter

[Prometheus Exporter](https://prometheus.io/docs/instrumenting/exporters/) for NVIDIA GPU device
metrics.

It uses the [Go bindings](https://github.com/NVIDIA/go-nvml) for the
[NVIDIA Management Library](https://developer.nvidia.com/nvidia-management-library-nvml) (NVML),
which is a C-based API that can be used for monitoring NVIDIA GPU devices.

> It does not use the [`nvidia-smi`](https://developer.nvidia.com/nvidia-system-management-interface) binary.

## `gpu_info` Time Series

This exporter adds also the `gpu_info` time series, which contains labels with useful GPU device
information, e.g. the `driver_version` on this host's device.

It adheres to one of the monitoring best practices described in more detail [here](https://www.robustperception.io/exposing-the-software-version-to-prometheus)
and adopted by other projects, e.g. [prometheus itself](https://github.com/prometheus/common/blob/main/version/info.go#L38)
and the [Go Prometheus collector](https://github.com/prometheus/client_golang/blob/main/prometheus/go_collector.go#L230).

## Build

Requirements:
- Go version >= 1.16

```shell
$ go build -o nvidia-gpu-exporter nvidia_gpu_exporter.go
```

## Run

Requirements:
- access to NVML library (`libnvidia-ml.so.1`)
- access to the GPU devices

```shell
$ LD_LIBRARY_PATH=${LD_LIBRARY_PATH}:<path-where-nvml-is-hosted> nvidia-gpu-exporter
```
