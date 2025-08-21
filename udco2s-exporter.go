package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/misodengaku/udco2s-exporter/udco2s"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	PromHTTPListenAddr string `json:"prometheus_listen_addr"`
	TTY                string `json:"tty"`
}

func main() {
	var err error
	var config Config
	config.PromHTTPListenAddr = os.Getenv("LISTEN_ADDR")
	if config.PromHTTPListenAddr == "" {
		panic("please specify LISTEN_ADDR environment variable")
	}
	config.TTY = os.Getenv("TTY")
	if config.TTY == "" {
		panic("please specify TTY environment variable")
	}

	co2Gauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "udco2s_co2_concentration",
		Help: "CO2 concentration",
	})
	humGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "udco2s_humidity",
		Help: "Humidity",
	})
	tempGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "udco2s_temperature",
		Help: "Temperature",
	})

	go func() {
		// promhttp
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(config.PromHTTPListenAddr, nil)
		slog.Error("failed to serve HTTP", "error", err)
	}()

	device := udco2s.UDCO2S{}
	err = device.Init(config.TTY)
	if err != nil {
		panic(err)
	}
	_ = device.StartMeasurement(context.Background())
	slog.Info("udco2s-exporter is running")

	for {
		co2Gauge.Set(float64(device.GetCO2Value()))
		humGauge.Set(device.GetHumidityValue())
		tempGauge.Set(device.GetTemperatureValue())
		time.Sleep(1 * time.Second)
	}
}
