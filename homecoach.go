package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

var (
	homecoachLabels = []string{"device_id", "device_name", "home_name"} 

	homecoachTemperatureDesc = prometheus.NewDesc(
		prefix+"homecoach_temperature",
		"Netatmo Home Coach measured temperature in degrees Celsius.",
		homecoachLabels,
		nil,
	)

	homecoachHumidityDesc = prometheus.NewDesc(
		prefix+"homecoach_humidity",
		"Netatmo Home Coach measured humidity in percent.",
		homecoachLabels,
		nil,
	)

	homecoachCO2Desc = prometheus.NewDesc(
		prefix+"homecoach_co2",
		"Netatmo Home Coach measured CO2 level in ppm.",
		homecoachLabels,
		nil,
	)

	homecoachNoiseDesc = prometheus.NewDesc(
		prefix+"homecoach_noise",
		"Netatmo Home Coach measured noise level in dB.",
		homecoachLabels,
		nil,
	)

	homecoachPressureDesc = prometheus.NewDesc(
		prefix+"homecoach_pressure",
		"Netatmo Home Coach measured pressure in mb.",
		homecoachLabels,
		nil,
	)

	homecoachHealthIndexDesc = prometheus.NewDesc(
		prefix+"homecoach_health_index",
		"Netatmo Home Coach health index (0: Healthy, 1: Fine, 2: Fair, 3: Poor, 4: Unhealthy).",
		homecoachLabels,
		nil,
	)
)

type HomeCoachCollector struct {
	log       logrus.FieldLogger
	tokenFunc func() (*oauth2.Token, error)
}

func NewHomeCoachCollector(log logrus.FieldLogger, tokenFunc func() (*oauth2.Token, error)) *HomeCoachCollector {
	return &HomeCoachCollector{
		log:       log,
		tokenFunc: tokenFunc,
	}
}

func (c *HomeCoachCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- homecoachTemperatureDesc
	ch <- homecoachHumidityDesc
	ch <- homecoachCO2Desc
	ch <- homecoachNoiseDesc
	ch <- homecoachPressureDesc
	ch <- homecoachHealthIndexDesc
}

// Collect implementiert prometheus.Collector
func (c *HomeCoachCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	token, err := c.tokenFunc()
	if err != nil {
		c.log.Errorf("HomeCoachCollector: error getting token: %v", err)
		return
	}
	if token == nil || !token.Valid() {
		c.log.Debug("HomeCoachCollector: token not available or invalid, skipping collection.")
		return
	}

	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	data, err := fetchHomeCoachData(ctx, httpClient)
	if err != nil {
		c.log.Errorf("HomeCoachCollector: error fetching data: %v", err)
		return
	}

	for _, device := range data.Body.Devices {
		labels := []string{device.ID, device.Name, device.HomeName}

		ch <- prometheus.MustNewConstMetric(
			homecoachTemperatureDesc,
			prometheus.GaugeValue,
			float64(device.DashboardData.Temperature),
			labels...,
		)

		ch <- prometheus.MustNewConstMetric(
			homecoachHumidityDesc,
			prometheus.GaugeValue,
			float64(device.DashboardData.Humidity),
			labels...,
		)

		ch <- prometheus.MustNewConstMetric(
			homecoachCO2Desc,
			prometheus.GaugeValue,
			float64(device.DashboardData.CO2),
			labels...,
		)

		ch <- prometheus.MustNewConstMetric(
			homecoachNoiseDesc,
			prometheus.GaugeValue,
			float64(device.DashboardData.Noise),
			labels...,
		)

		ch <- prometheus.MustNewConstMetric(
			homecoachPressureDesc,
			prometheus.GaugeValue,
			float64(device.DashboardData.Pressure),
			labels...,
		)

		ch <- prometheus.MustNewConstMetric(
			homecoachHealthIndexDesc,
			prometheus.GaugeValue,
			float64(device.DashboardData.HealthIndex),
			labels...,
		)
	}
}

type homeCoachResponse struct {
	Body struct {
		Devices []struct {
			ID        string `json:"_id"`
			Name      string `json:"name"`
			HomeName  string `json:"home_name"`
			Type      string `json:"type"`
			DashboardData struct {
				Temperature float32 `json:"Temperature"`
				CO2        int32   `json:"CO2"`
				Humidity   int32   `json:"Humidity"`
				Noise      int32   `json:"Noise"`
				Pressure   float32 `json:"Pressure"`
				HealthIndex int32  `json:"health_idx"`
			} `json:"dashboard_data"`
		} `json:"devices"`
	} `json:"body"`
}

func fetchHomeCoachData(ctx context.Context, client *http.Client) (*homeCoachResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.netatmo.com/api/gethomecoachsdata", nil)
	if err != nil {
		return nil, fmt.Errorf("creating gethomecoachsdata request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing gethomecoachsdata request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gethomecoachsdata request failed: status %s", resp.Status)
	}

	var result homeCoachResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding gethomecoachsdata response: %w", err)
	}

	return &result, nil
}