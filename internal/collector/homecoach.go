package collector

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/sirupsen/logrus"
)

var (
    // HomeCoach specific labels
    homecoachLabels = []string{"device_id", "device_name"}

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

// HomeCoachReadFunction definiert das Interface zum Lesen der HomeCoach Daten von der Netatmo API.
type HomeCoachReadFunction func() (*HomeCoachResponse, error)

type HomeCoachCollector struct {
    log             logrus.FieldLogger
    readFunction    HomeCoachReadFunction
    RefreshInterval time.Duration
    StaleThreshold  time.Duration
    clock           func() time.Time

    lastRefresh         time.Time
    lastRefreshError    error
    lastRefreshDuration time.Duration

    cacheLock      sync.RWMutex
    cacheTimestamp time.Time
    cachedData     *HomeCoachResponse
}

func NewHomeCoachCollector(log logrus.FieldLogger, readFunction HomeCoachReadFunction, refreshInterval, staleDuration time.Duration) *HomeCoachCollector {
    return &HomeCoachCollector{
        log:             log,
        readFunction:    readFunction,
        RefreshInterval: refreshInterval,
        StaleThreshold:  staleDuration,
        clock:           time.Now,
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

func (c *HomeCoachCollector) Collect(ch chan<- prometheus.Metric) {
    now := c.clock()
    if now.Sub(c.lastRefresh) >= c.RefreshInterval {
        go c.refreshData(now)
    }

    c.cacheLock.RLock()
    defer c.cacheLock.RUnlock()

    if c.cachedData == nil {
        return
    }

    for _, device := range c.cachedData.Body.Devices {
        // Nur device_id und device_name
        labels := []string{device.ID, device.StationName}

        c.sendMetric(ch, homecoachTemperatureDesc, prometheus.GaugeValue, float64(device.DashboardData.Temperature), labels...)
        c.sendMetric(ch, homecoachHumidityDesc, prometheus.GaugeValue, float64(device.DashboardData.Humidity), labels...)
        c.sendMetric(ch, homecoachCO2Desc, prometheus.GaugeValue, float64(device.DashboardData.CO2), labels...)
        c.sendMetric(ch, homecoachNoiseDesc, prometheus.GaugeValue, float64(device.DashboardData.Noise), labels...)
        c.sendMetric(ch, homecoachPressureDesc, prometheus.GaugeValue, float64(device.DashboardData.Pressure), labels...)
        c.sendMetric(ch, homecoachHealthIndexDesc, prometheus.GaugeValue, float64(device.DashboardData.HealthIndex), labels...)
    }
}

func (c *HomeCoachCollector) refreshData(now time.Time) {
    c.log.Debugf("HomeCoachCollector: refreshing data. Time since last refresh: %s", now.Sub(c.lastRefresh))
    c.lastRefresh = now

    defer func(start time.Time) {
        c.lastRefreshDuration = c.clock().Sub(start)
    }(c.clock())

    data, err := c.readFunction()
    c.lastRefreshError = err
    if err != nil {
        c.log.Errorf("HomeCoachCollector: error during refresh: %s", err)
        return
    }

    c.cacheLock.Lock()
    c.cacheTimestamp = now
    c.cachedData = data
    c.cacheLock.Unlock()
}

func (c *HomeCoachCollector) sendMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, value float64, labelValues ...string) {
    m, err := prometheus.NewConstMetric(desc, valueType, value, labelValues...)
    if err != nil {
        c.log.Errorf("HomeCoachCollector: error creating metric %s: %v", desc.String(), err)
        return
    }
    ch <- m
}

type HomeCoachResponse struct {
    Body struct {
        Devices []struct {
            ID           string `json:"_id"`
            StationName  string `json:"station_name"`
            Type         string `json:"type"`
            DashboardData struct {
                Temperature float32 `json:"Temperature"`
                CO2        int32   `json:"CO2"`
                Humidity   int32   `json:"Humidity"`
                Noise     int32   `json:"Noise"`
                Pressure  float32 `json:"Pressure"`
                HealthIndex int32  `json:"health_idx"`
            } `json:"dashboard_data"`
        } `json:"devices"`
    } `json:"body"`
}

func FetchHomeCoachData(client *http.Client) (*HomeCoachResponse, error) {
    req, err := http.NewRequest(http.MethodGet, "https://api.netatmo.com/api/gethomecoachsdata", nil)
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

    var result HomeCoachResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decoding gethomecoachsdata response: %w", err)
    }

    return &result, nil
}
