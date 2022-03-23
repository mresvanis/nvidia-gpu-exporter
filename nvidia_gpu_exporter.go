package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "nvidia_gpu"
)

var (
	deviceLabels = []string{"minor_number", "uuid", "name"}

	gpuInfoLabels = []string{"driver_version"}
)

type Exporter struct {
	sync.Mutex
	logger log.Logger

	gpuInfo     *prometheus.Desc
	numDevices  prometheus.Gauge
	usedMemory  *prometheus.GaugeVec
	totalMemory *prometheus.GaugeVec
	dutyCycle   *prometheus.GaugeVec
	powerUsage  *prometheus.GaugeVec
	temperature *prometheus.GaugeVec
	fanSpeed    *prometheus.GaugeVec
}

func NewExporter(logger log.Logger) *Exporter {
	return &Exporter{
		logger: logger,
		gpuInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "gpu_info"),
			fmt.Sprintf("A metric with a constant '1' value labeled by gpu %s.", strings.Join(gpuInfoLabels, ", ")),
			gpuInfoLabels,
			nil,
		),
		numDevices: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "num_devices",
				Help:      "Number of GPU devices",
			},
		),
		usedMemory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_used_bytes",
				Help:      "Memory used by the GPU device in bytes",
			},
			deviceLabels,
		),
		totalMemory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_total_bytes",
				Help:      "Total memory of the GPU device in bytes",
			},
			deviceLabels,
		),
		dutyCycle: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "duty_cycle",
				Help:      "Percent of time over the past sample period during which one or more kernels were executing on the GPU device",
			},
			deviceLabels,
		),
		powerUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "power_usage_milliwatts",
				Help:      "Power usage of the GPU device in milliwatts",
			},
			deviceLabels,
		),
		temperature: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "temperature_celsius",
				Help:      "Temperature of the GPU device in celsius",
			},
			deviceLabels,
		),
		fanSpeed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "fanspeed_percent",
				Help:      "Fanspeed of the GPU device as a percent of its maximum",
			},
			deviceLabels,
		),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.gpuInfo
	ch <- e.numDevices.Desc()

	e.usedMemory.Describe(ch)
	e.totalMemory.Describe(ch)
	e.dutyCycle.Describe(ch)
	e.powerUsage.Describe(ch)
	e.temperature.Describe(ch)
	e.fanSpeed.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.Lock()
	defer e.Unlock()

	e.usedMemory.Reset()
	e.totalMemory.Reset()
	e.dutyCycle.Reset()
	e.powerUsage.Reset()
	e.temperature.Reset()
	e.fanSpeed.Reset()

	driverVersion, ret := nvml.SystemGetDriverVersion()
	if ret != nvml.SUCCESS {
		level.Error(e.logger).Log("msg", "Unable to get system driver version", "err", nvml.ErrorString(ret))
	} else {
		ch <- prometheus.MustNewConstMetric(e.gpuInfo, prometheus.GaugeValue, 1, driverVersion)
	}

	numDevices, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		level.Error(e.logger).Log("msg", "Unable to get device count", "err", nvml.ErrorString(ret))
		return
	}
	e.numDevices.Set(float64(numDevices))
	ch <- e.numDevices

	for i := 0; i < int(numDevices); i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get device", "index", i, "err", nvml.ErrorString(ret))
			continue
		}

		minorNumber, ret := device.GetMinorNumber()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get minor number of device", "index", i, "err", nvml.ErrorString(ret))
			continue
		}
		minor := strconv.Itoa(int(minorNumber))

		uuid, ret := device.GetUUID()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get UUID of device", "index", i, "err", nvml.ErrorString(ret))
			continue
		}

		name, ret := device.GetName()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get name of device", "index", i, "err", nvml.ErrorString(ret))
			continue
		}

		memoryInfo, ret := device.GetMemoryInfo()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get memory info of device", "index", i, "err", nvml.ErrorString(ret))

		} else {
			e.usedMemory.WithLabelValues(minor, uuid, name).Set(float64(memoryInfo.Used))
			e.totalMemory.WithLabelValues(minor, uuid, name).Set(float64(memoryInfo.Total))
		}

		utilization, ret := device.GetUtilizationRates()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get utilization rates of device", "index", i, "err", nvml.ErrorString(ret))

		} else {
			e.dutyCycle.WithLabelValues(minor, uuid, name).Set(float64(utilization.Gpu))
		}

		powerUsage, ret := device.GetPowerUsage()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get power usage of device", "index", i, "err", nvml.ErrorString(ret))

		} else {
			e.powerUsage.WithLabelValues(minor, uuid, name).Set(float64(powerUsage))
		}

		temperature, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get temperature of device", "index", i, "err", nvml.ErrorString(ret))

		} else {
			e.temperature.WithLabelValues(minor, uuid, name).Set(float64(temperature))
		}

		fanSpeed, ret := device.GetFanSpeed()
		if ret != nvml.SUCCESS {
			level.Error(e.logger).Log("msg", "Unable to get fan speed of device", "index", i, "err", nvml.ErrorString(ret))

		} else {
			e.fanSpeed.WithLabelValues(minor, uuid, name).Set(float64(fanSpeed))
		}
	}

	e.usedMemory.Collect(ch)
	e.totalMemory.Collect(ch)
	e.dutyCycle.Collect(ch)
	e.powerUsage.Collect(ch)
	e.temperature.Collect(ch)
	e.fanSpeed.Collect(ch)
}

func main() {
	var (
		webConfig     = webflag.AddFlags(kingpin.CommandLine)
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9445").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("nvidia_gpu_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	if ret := nvml.Init(); ret != nvml.SUCCESS {
		level.Error(logger).Log("msg", "Couldn't initialize NVML. Make sure NVML is in the shared library search path.", "err", nvml.ErrorString(ret))
		os.Exit(1)
	}
	defer func() {
		if ret := nvml.Shutdown(); ret != nvml.SUCCESS {
			level.Error(logger).Log("msg", "Unable to shutdown NVML", "err", nvml.ErrorString(ret))
		}
	}()

	if driverVersion, ret := nvml.SystemGetDriverVersion(); ret != nvml.SUCCESS {
		level.Error(logger).Log("msg", "Unable to get system driver version", "err", nvml.ErrorString(ret))
	} else {
		level.Info(logger).Log("msg", "System driver:", "version", driverVersion)
	}

	prometheus.MustRegister(NewExporter(logger))
	prometheus.MustRegister(version.NewCollector("nvidia_gpu_exporter"))

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
									 <head><title>NVIDIA GPU Exporter</title></head>
									 <body>
									 <h1>NVIDIA GPU Exporter</h1>
									 <p><a href='` + *metricsPath + `'>Metrics</a></p>
									 </body>
									 </html>`))
	})
	srv := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(srv, *webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
