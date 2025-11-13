package collector

import (
	"sync"
	"time"

	netatmo "github.com/exzz/netatmo-api-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	// Weather station specific labels
	weatherLabels = []string{"module", "station", "home"}

	netatmoUpDesc = prometheus.NewDesc(prefix+"up",
		"Zero if there was an error during the last refresh try.",
		nil, nil)

	refreshIntervalDesc = prometheus.NewDesc(
		prefix+"refresh_interval_seconds",
		"Contains the configured refresh interval in seconds. This is provided as a convenience for calculations with the cache update time.",
		nil, nil)
	refreshPrefix        = prefix + "last_refresh_"
	refreshTimestampDesc = prometheus.NewDesc(
		refreshPrefix+"time",
		"Contains the time of the last refresh try, successful or not.",
		nil, nil)
	refreshDurationDesc = prometheus.NewDesc(
		refreshPrefix+"duration_seconds",
		"Contains the time it took for the last refresh to complete, even if it was unsuccessful.",
		nil, nil)

	cacheTimestampDesc = prometheus.NewDesc(
		prefix+"cache_updated_time",
		"Contains the time of the cached data.",
		nil, nil)

	sensorPrefix = prefix + "sensor_"

	updatedDesc = prometheus.NewDesc(
		sensorPrefix+"updated",
		"Timestamp of last update",
		weatherLabels,
		nil)

	tempDesc = prometheus.NewDesc(
		sensorPrefix+"temperature_celsius",
		"Temperature measurement in celsius",
		weatherLabels,
		nil)

	humidityDesc = prometheus.NewDesc(
		sensorPrefix+"humidity_percent",
		"Relative humidity measurement in percent",
		weatherLabels,
		nil)

	cotwoDesc = prometheus.NewDesc(
		sensorPrefix+"co2_ppm",
		"Carbondioxide measurement in parts per million",
		weatherLabels,
		nil)

	noiseDesc = prometheus.NewDesc(
		sensorPrefix+"noise_db",
		"Noise measurement in decibels",
		weatherLabels,
		nil)

	pressureDesc = prometheus.NewDesc(
		sensorPrefix+"pressure_mb",
		"Atmospheric pressure measurement in millibar",
		weatherLabels,
		nil)

	windStrengthDesc = prometheus.NewDesc(
		sensorPrefix+"wind_strength_kph",
		"Wind strength in kilometers per hour",
		weatherLabels,
		nil)

	windDirectionDesc = prometheus.NewDesc(
		sensorPrefix+"wind_direction_degrees",
		"Wind direction in degrees",
		weatherLabels,
		nil)

	rainDesc = prometheus.NewDesc(
		sensorPrefix+"rain_amount_mm",
		"Rain amount in millimeters",
		weatherLabels,
		nil)

	batteryDesc = prometheus.NewDesc(
		sensorPrefix+"battery_percent",
		"Battery remaining life (10: low)",
		weatherLabels,
		nil)
	wifiDesc = prometheus.NewDesc(
		sensorPrefix+"wifi_signal_strength",
		"Wifi signal strength (86: bad, 71: avg, 56: good)",
		weatherLabels,
		nil)
	rfDesc = prometheus.NewDesc(
		sensorPrefix+"rf_signal_strength",
		"RF signal strength (90: lowest, 60: highest)",
		weatherLabels,
		nil)
)

// ReadFunction defines the interface for reading from the Netatmo API.
type ReadFunction func() (*netatmo.DeviceCollection, error)

// NetatmoCollector is a Prometheus collector for Netatmo sensor values.
type NetatmoCollector struct {
	Log             logrus.FieldLogger
	RefreshInterval time.Duration
	StaleThreshold  time.Duration
	ReadFunction    ReadFunction
	clock           func() time.Time

	lastRefresh         time.Time
	lastRefreshError    error
	lastRefreshDuration time.Duration
	cacheLock           sync.RWMutex
	cacheTimestamp      time.Time
	cachedData          *netatmo.DeviceCollection
}

func New(log *logrus.Logger, readFunction ReadFunction, refreshInterval, staleDuration time.Duration) *NetatmoCollector {
	return &NetatmoCollector{
		Log:             log,
		RefreshInterval: refreshInterval,
		StaleThreshold:  staleDuration,
		ReadFunction:    readFunction,
		clock:           time.Now,
	}
}

// Describe implements prometheus.Collector
func (c *NetatmoCollector) Describe(dChan chan<- *prometheus.Desc) {
	dChan <- netatmoUpDesc
	dChan <- refreshIntervalDesc
	dChan <- refreshTimestampDesc
	dChan <- refreshDurationDesc
	dChan <- cacheTimestampDesc
	dChan <- updatedDesc
	dChan <- tempDesc
	dChan <- humidityDesc
	dChan <- cotwoDesc
	dChan <- noiseDesc
	dChan <- pressureDesc
	dChan <- windStrengthDesc
	dChan <- windDirectionDesc
	dChan <- rainDesc
	dChan <- batteryDesc
	dChan <- wifiDesc
	dChan <- rfDesc
}

// Collect implements prometheus.Collector
func (c *NetatmoCollector) Collect(mChan chan<- prometheus.Metric) {
	now := c.clock()
	if now.Sub(c.lastRefresh) >= c.RefreshInterval {
		go c.RefreshData(now)
	}

	upValue := 1.0
	if c.lastRefresh.IsZero() || c.lastRefreshError != nil {
		upValue = 0
	}
	sendMetric(c.Log, mChan, netatmoUpDesc, prometheus.GaugeValue, upValue)
	sendMetric(c.Log, mChan, refreshIntervalDesc, prometheus.GaugeValue, c.RefreshInterval.Seconds())
	sendMetric(c.Log, mChan, refreshTimestampDesc, prometheus.GaugeValue, convertTime(c.lastRefresh))
	sendMetric(c.Log, mChan, refreshDurationDesc, prometheus.GaugeValue, c.lastRefreshDuration.Seconds())

	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	sendMetric(c.Log, mChan, cacheTimestampDesc, prometheus.GaugeValue, convertTime(c.cacheTimestamp))
	if c.cachedData != nil {
		for _, dev := range c.cachedData.Devices() {
			homeName := dev.HomeName
			stationName := dev.StationName //nolint: staticcheck
			c.collectData(mChan, dev, stationName, homeName)

			for _, module := range dev.LinkedModules {
				c.collectData(mChan, module, stationName, homeName)
			}
		}
	}
}

// RefreshData causes the collector to try to refresh the cached data.
func (c *NetatmoCollector) RefreshData(now time.Time) {
	c.Log.Debugf("Refreshing data. Time since last refresh: %s", now.Sub(c.lastRefresh))
	c.lastRefresh = now

	defer func(start time.Time) {
		c.lastRefreshDuration = c.clock().Sub(start)
	}(c.clock())

	devices, err := c.ReadFunction()
	c.lastRefreshError = err
	if err != nil {
		c.Log.Errorf("Error during refresh: %s", err)
		return
	}

	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	c.cacheTimestamp = now
	c.cachedData = devices
}

func (c *NetatmoCollector) collectData(ch chan<- prometheus.Metric, device *netatmo.Device, stationName, homeName string) {
	moduleName := device.ModuleName
	if moduleName == "" {
		moduleName = "id-" + device.ID
	}

	data := device.DashboardData

	if data.LastMeasure == nil {
		c.Log.Debugf("No data available.")
		return
	}

	date := time.Unix(*data.LastMeasure, 0)
	dataAge := c.clock().Sub(date)
	if dataAge > c.StaleThreshold {
		c.Log.Debugf("Data is stale for %s: %s > %s", moduleName, dataAge, c.StaleThreshold)
		return
	}

	sendMetric(c.Log, ch, updatedDesc, prometheus.GaugeValue, float64(date.UTC().Unix()), moduleName, stationName, homeName)

	if data.Temperature != nil {
		sendMetric(c.Log, ch, tempDesc, prometheus.GaugeValue, float64(*data.Temperature), moduleName, stationName, homeName)
	}

	if data.Humidity != nil {
		sendMetric(c.Log, ch, humidityDesc, prometheus.GaugeValue, float64(*data.Humidity), moduleName, stationName, homeName)
	}

	if data.CO2 != nil {
		sendMetric(c.Log, ch, cotwoDesc, prometheus.GaugeValue, float64(*data.CO2), moduleName, stationName, homeName)
	}

	if data.Noise != nil {
		sendMetric(c.Log, ch, noiseDesc, prometheus.GaugeValue, float64(*data.Noise), moduleName, stationName, homeName)
	}

	if data.Pressure != nil {
		sendMetric(c.Log, ch, pressureDesc, prometheus.GaugeValue, float64(*data.Pressure), moduleName, stationName, homeName)
	}

	if data.WindStrength != nil {
		sendMetric(c.Log, ch, windStrengthDesc, prometheus.GaugeValue, float64(*data.WindStrength), moduleName, stationName, homeName)
	}

	if data.WindAngle != nil {
		sendMetric(c.Log, ch, windDirectionDesc, prometheus.GaugeValue, float64(*data.WindAngle), moduleName, stationName, homeName)
	}

	if data.Rain != nil {
		sendMetric(c.Log, ch, rainDesc, prometheus.GaugeValue, float64(*data.Rain), moduleName, stationName, homeName)
	}

	if device.BatteryPercent != nil {
		sendMetric(c.Log, ch, batteryDesc, prometheus.GaugeValue, float64(*device.BatteryPercent), moduleName, stationName, homeName)
	}
	if device.WifiStatus != nil {
		sendMetric(c.Log, ch, wifiDesc, prometheus.GaugeValue, float64(*device.WifiStatus), moduleName, stationName, homeName)
	}
	if device.RFStatus != nil {
		sendMetric(c.Log, ch, rfDesc, prometheus.GaugeValue, float64(*device.RFStatus), moduleName, stationName, homeName)
	}
}
