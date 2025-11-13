package collector

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	prefix = "netatmo_"
)

func convertTime(t time.Time) float64 {
	if t.IsZero() {
		return 0.0
	}
	return float64(t.Unix())
}

func sendMetric(log logrus.FieldLogger, ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, value float64, labelValues ...string) {
	m, err := prometheus.NewConstMetric(desc, valueType, value, labelValues...)
	if err != nil {
		log.Errorf("Error creating metric %s: %v", desc.String(), err)
		return
	}
	ch <- m
}
