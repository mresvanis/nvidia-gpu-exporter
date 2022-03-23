ARG ARCH="amd64"
ARG OS="linux"

FROM golang:1.17 as builder

WORKDIR /opt/nvidia/src

COPY go.mod go.mod
COPY go.sum go.sum
COPY nvidia_gpu_exporter.go nvidia_gpu_exporter.go

RUN go mod download

RUN GOOS=${OS} GOARCH=${ARCH} go build -o nvidia_gpu_exporter nvidia_gpu_exporter.go

FROM ubuntu:20.04

ARG ARCH="amd64"
ARG OS="linux"
COPY --from=builder /opt/nvidia/src/nvidia_gpu_exporter /bin/nvidia_gpu_exporter

EXPOSE      9445
ENTRYPOINT  ["/bin/nvidia_gpu_exporter"]
