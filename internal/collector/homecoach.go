package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

var (
	// HomeCoach specific labels
	homecoachLabels = []string{"device_id", "device_name"}

	// HomeCoach collector status metrics
	homecoachUpDesc = prometheus.NewDesc(
		prefix+"homecoach_up",
		"Zero if there was an error during the last refresh try.",
		nil, nil,
	)

	homecoachRefreshIntervalDesc = prometheus.NewDesc(
		prefix+"homecoach_refresh_interval_seconds",
		"Contains the configured refresh interval in seconds. This is provided as a convenience for calculations with the cache update time.",
		nil, nil,
	)

	homecoachRefreshPrefix        = prefix + "homecoach_last_refresh_"
	homecoachRefreshTimestampDesc = prometheus.NewDesc(
		homecoachRefreshPrefix+"time",
		"Contains the time of the last refresh try, successful or not.",
		nil, nil,
	)
	homecoachRefreshDurationDesc = prometheus.NewDesc(
		homecoachRefreshPrefix+"duration_seconds",
		"Contains the time it took for the last refresh to complete, even if it was unsuccessful.",
		nil, nil,
	)

	homecoachCacheTimestampDesc = prometheus.NewDesc(
		prefix+"homecoach_cache_updated_time",
		"Contains the time of the cached data.",
		nil, nil,
	)

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

	homecoachWifiDesc = prometheus.NewDesc(
		prefix+"homecoach_wifi_signal_strength",
		"Wifi signal strength (86: bad, 71: avg, 56: good).",
		homecoachLabels,
		nil,
	)
)

// HomecoachReadFunction defines the interface for reading HomeCoach data from the Netatmo API.
type HomecoachReadFunction func() (*HomecoachResponse, error)

type HomeCoachCollector struct {
	log             logrus.FieldLogger
	readFunction    HomecoachReadFunction
	RefreshInterval time.Duration
	StaleThreshold  time.Duration
	clock           func() time.Time

	lastRefresh         time.Time
	lastRefreshError    error
	lastRefreshDuration time.Duration

	cacheLock      sync.RWMutex
	cacheTimestamp time.Time
	cachedData     *HomecoachResponse
}

func NewHomecoachCollector(log logrus.FieldLogger, readFunction HomecoachReadFunction, refreshInterval, staleDuration time.Duration) *HomeCoachCollector {
	return &HomeCoachCollector{
		log:             log,
		readFunction:    readFunction,
		RefreshInterval: refreshInterval,
		StaleThreshold:  staleDuration,
		clock:           time.Now,
	}
}

func (c *HomeCoachCollector) Describe(ch chan<- *prometheus.Desc) {
	// Status metrics
	ch <- homecoachUpDesc
	ch <- homecoachRefreshIntervalDesc
	ch <- homecoachRefreshTimestampDesc
	ch <- homecoachRefreshDurationDesc
	ch <- homecoachCacheTimestampDesc

	// Data metrics
	ch <- homecoachTemperatureDesc
	ch <- homecoachHumidityDesc
	ch <- homecoachCO2Desc
	ch <- homecoachNoiseDesc
	ch <- homecoachPressureDesc
	ch <- homecoachHealthIndexDesc
	ch <- homecoachWifiDesc
}

func (c *HomeCoachCollector) Collect(ch chan<- prometheus.Metric) {
	now := c.clock()
	if now.Sub(c.lastRefresh) >= c.RefreshInterval {
		go c.refreshData(now)
	}

	upValue := 1.0
	if c.lastRefresh.IsZero() || c.lastRefreshError != nil {
		upValue = 0
	}

	sendMetric(c.log, ch, homecoachUpDesc, prometheus.GaugeValue, upValue)
	sendMetric(c.log, ch, homecoachRefreshIntervalDesc, prometheus.GaugeValue, c.RefreshInterval.Seconds())
	sendMetric(c.log, ch, homecoachRefreshTimestampDesc, prometheus.GaugeValue, convertTime(c.lastRefresh))
	sendMetric(c.log, ch, homecoachRefreshDurationDesc, prometheus.GaugeValue, c.lastRefreshDuration.Seconds())

	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	sendMetric(c.log, ch, homecoachCacheTimestampDesc, prometheus.GaugeValue, convertTime(c.cacheTimestamp))
	if c.cachedData == nil {
		return
	}

	for _, device := range c.cachedData.Body.Devices {
		// only device_id and device_name
		labels := []string{device.ID, device.StationName}

		sendMetric(c.log, ch, homecoachTemperatureDesc, prometheus.GaugeValue, float64(device.DashboardData.Temperature), labels...)
		sendMetric(c.log, ch, homecoachHumidityDesc, prometheus.GaugeValue, float64(device.DashboardData.Humidity), labels...)
		sendMetric(c.log, ch, homecoachCO2Desc, prometheus.GaugeValue, float64(device.DashboardData.CO2), labels...)
		sendMetric(c.log, ch, homecoachNoiseDesc, prometheus.GaugeValue, float64(device.DashboardData.Noise), labels...)
		sendMetric(c.log, ch, homecoachPressureDesc, prometheus.GaugeValue, float64(device.DashboardData.Pressure), labels...)
		sendMetric(c.log, ch, homecoachHealthIndexDesc, prometheus.GaugeValue, float64(device.DashboardData.HealthIndex), labels...)
		sendMetric(c.log, ch, homecoachWifiDesc, prometheus.GaugeValue, float64(device.WifiStatus), labels...)
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

type HomecoachResponse struct {
	Body struct {
		Devices []struct {
			ID              string   `json:"_id"`
			DateSetup       int64    `json:"date_setup"`
			LastSetup       int64    `json:"last_setup"`
			Type            string   `json:"type"`
			LastStatusStore int64    `json:"last_status_store"`
			ModuleName      string   `json:"module_name"`
			Firmware        int      `json:"firmware"`
			LastUpgrade     int64    `json:"last_upgrade"`
			WifiStatus      int      `json:"wifi_status"`
			Reachable       bool     `json:"reachable"`
			CO2Calibrating  bool     `json:"co2_calibrating"`
			StationName     string   `json:"station_name"`
			DataType        []string `json:"data_type"`
			Place           struct {
				Altitude int       `json:"altitude"`
				City     string    `json:"city"`
				Country  string    `json:"country"`
				Timezone string    `json:"timezone"`
				Location []float64 `json:"location"`
			} `json:"place"`
			DashboardData struct {
				TimeUTC          int64   `json:"time_utc"`
				Temperature      float32 `json:"Temperature"`
				CO2              int32   `json:"CO2"`
				Humidity         int32   `json:"Humidity"`
				Noise            int32   `json:"Noise"`
				Pressure         float32 `json:"Pressure"`
				AbsolutePressure float32 `json:"AbsolutePressure"`
				HealthIndex      int32   `json:"health_idx"`
				MinTemp          float32 `json:"min_temp"`
				MaxTemp          float32 `json:"max_temp"`
				DateMaxTemp      int64   `json:"date_max_temp"`
				DateMinTemp      int64   `json:"date_min_temp"`
			} `json:"dashboard_data"`
			Name     string `json:"name"`
			ReadOnly bool   `json:"read_only"`
		} `json:"devices"`
		User struct {
			Mail           string `json:"mail"`
			Administrative struct {
				Lang         string `json:"lang"`
				RegLocale    string `json:"reg_locale"`
				Country      string `json:"country"`
				Unit         int    `json:"unit"`
				Windunit     int    `json:"windunit"`
				Pressureunit int    `json:"pressureunit"`
				FeelLikeAlgo int    `json:"feel_like_algo"`
			} `json:"administrative"`
		} `json:"user"`
	} `json:"body"`
}

func FetchHomecoachData(client *http.Client) (*HomecoachResponse, error) {
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

	var result HomecoachResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding gethomecoachsdata response: %w", err)
	}

	return &result, nil
}

// NewHomecoachReadFunction creates a reader function for HomeCoach data
func NewHomecoachReadFunction(getCurrentToken func() (*oauth2.Token, error)) HomecoachReadFunction {
	return func() (*HomecoachResponse, error) {
		token, err := getCurrentToken()
		if err != nil {
			return nil, fmt.Errorf("getting token: %w", err)
		}
		if token == nil || !token.Valid() {
			return nil, fmt.Errorf("token not available or invalid")
		}
		httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
		return FetchHomecoachData(httpClient)
	}
}
