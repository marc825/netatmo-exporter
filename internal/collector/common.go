package collector

import (
	"sync"
	"time"

	netatmo "github.com/exzz/netatmo-api-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	prefix       = "netatmo_"
	sensorPrefix = prefix + "sensor_"
)

// V2 unified label names
var v2LabelNames = []string{"device_class", "device_id", "home", "module", "station"}

// V2 unified metric descriptors
var (
	// Sensor data metrics
	v2UpdatedDesc       = prometheus.NewDesc(sensorPrefix+"updated", "Timestamp of last update", v2LabelNames, nil)
	v2TempDesc          = prometheus.NewDesc(sensorPrefix+"temperature_celsius", "Temperature measurement in celsius", v2LabelNames, nil)
	v2HumidityDesc      = prometheus.NewDesc(sensorPrefix+"humidity_percent", "Relative humidity measurement in percent", v2LabelNames, nil)
	v2CO2Desc           = prometheus.NewDesc(sensorPrefix+"co2_ppm", "Carbondioxide measurement in parts per million", v2LabelNames, nil)
	v2NoiseDesc         = prometheus.NewDesc(sensorPrefix+"noise_db", "Noise measurement in decibels", v2LabelNames, nil)
	v2PressureDesc      = prometheus.NewDesc(sensorPrefix+"pressure_mb", "Atmospheric pressure measurement in millibar", v2LabelNames, nil)
	v2RainDesc          = prometheus.NewDesc(sensorPrefix+"rain_amount_mm", "Rain amount in millimeters", v2LabelNames, nil)
	v2WindStrengthDesc  = prometheus.NewDesc(sensorPrefix+"wind_strength_kph", "Wind strength in kilometers per hour", v2LabelNames, nil)
	v2WindDirectionDesc = prometheus.NewDesc(sensorPrefix+"wind_direction_degrees", "Wind direction in degrees", v2LabelNames, nil)
	v2BatteryDesc       = prometheus.NewDesc(sensorPrefix+"battery_percent", "Battery remaining life (10: low)", v2LabelNames, nil)
	v2WifiDesc          = prometheus.NewDesc(sensorPrefix+"wifi_signal_strength", "Wifi signal strength (86: bad, 71: avg, 56: good)", v2LabelNames, nil)
	v2RFDesc            = prometheus.NewDesc(sensorPrefix+"rf_signal_strength", "RF signal strength (90: lowest, 60: highest)", v2LabelNames, nil)
	v2HealthIndexDesc   = prometheus.NewDesc(sensorPrefix+"health_index", "Air quality health index (0: Healthy, 1: Fine, 2: Fair, 3: Poor, 4: Unhealthy)", v2LabelNames, nil)

	// Weather meta metrics
	v2WeatherUpDesc               = prometheus.NewDesc(prefix+"up", "Zero if there was an error during the last refresh try.", nil, nil)
	v2WeatherRefreshIntervalDesc  = prometheus.NewDesc(prefix+"refresh_interval_seconds", "Contains the configured refresh interval in seconds. This is provided as a convenience for calculations with the cache update time.", nil, nil)
	v2WeatherRefreshTimestampDesc = prometheus.NewDesc(prefix+"last_refresh_time", "Contains the time of the last refresh try, successful or not.", nil, nil)
	v2WeatherRefreshDurationDesc  = prometheus.NewDesc(prefix+"last_refresh_duration_seconds", "Contains the time it took for the last refresh to complete, even if it was unsuccessful.", nil, nil)
	v2WeatherCacheTimestampDesc   = prometheus.NewDesc(prefix+"cache_updated_time", "Contains the time of the cached data.", nil, nil)

	// Homecoach meta metrics
	v2HomecoachUpDesc               = prometheus.NewDesc(prefix+"homecoach_up", "Zero if there was an error during the last refresh try.", nil, nil)
	v2HomecoachRefreshIntervalDesc  = prometheus.NewDesc(prefix+"homecoach_refresh_interval_seconds", "Contains the configured refresh interval in seconds. This is provided as a convenience for calculations with the cache update time.", nil, nil)
	v2HomecoachRefreshTimestampDesc = prometheus.NewDesc(prefix+"homecoach_last_refresh_time", "Contains the time of the last refresh try, successful or not.", nil, nil)
	v2HomecoachRefreshDurationDesc  = prometheus.NewDesc(prefix+"homecoach_last_refresh_duration_seconds", "Contains the time it took for the last refresh to complete, even if it was unsuccessful.", nil, nil)
	v2HomecoachCacheTimestampDesc   = prometheus.NewDesc(prefix+"homecoach_cache_updated_time", "Contains the time of the cached data.", nil, nil)
)

// UnifiedCollectorV2 combines Weather and Homecoach data with unified labels
type UnifiedCollectorV2 struct {
	log             logrus.FieldLogger
	weatherReader   WeatherReadFunction
	homecoachReader HomecoachReadFunction
	refreshInterval time.Duration
	staleThreshold  time.Duration
	clock           func() time.Time
	enableWeather   bool
	enableHomecoach bool

	weatherLock                sync.RWMutex
	weatherLastRefresh         time.Time
	weatherLastRefreshError    error
	weatherLastRefreshDuration time.Duration
	weatherCachedData          *netatmo.DeviceCollection

	homecoachLock                sync.RWMutex
	homecoachLastRefresh         time.Time
	homecoachLastRefreshError    error
	homecoachLastRefreshDuration time.Duration
	homecoachCachedData          *HomecoachResponse
}

func UnifiedCollector(
	log logrus.FieldLogger,
	weatherReader WeatherReadFunction,
	homecoachReader HomecoachReadFunction,
	refreshInterval, staleThreshold time.Duration,
	enableWeather, enableHomecoach bool,
) *UnifiedCollectorV2 {
	return &UnifiedCollectorV2{
		log:             log,
		weatherReader:   weatherReader,
		homecoachReader: homecoachReader,
		refreshInterval: refreshInterval,
		staleThreshold:  staleThreshold,
		clock:           time.Now,
		enableWeather:   enableWeather,
		enableHomecoach: enableHomecoach,
	}
}

func (c *UnifiedCollectorV2) Describe(ch chan<- *prometheus.Desc) {
	// Sensor data descriptors
	ch <- v2UpdatedDesc
	ch <- v2TempDesc
	ch <- v2HumidityDesc
	ch <- v2CO2Desc
	ch <- v2NoiseDesc
	ch <- v2PressureDesc
	ch <- v2RainDesc
	ch <- v2WindStrengthDesc
	ch <- v2WindDirectionDesc
	ch <- v2BatteryDesc
	ch <- v2WifiDesc
	ch <- v2RFDesc
	ch <- v2HealthIndexDesc

	// Weather meta descriptors
	if c.enableWeather {
		ch <- v2WeatherUpDesc
		ch <- v2WeatherRefreshIntervalDesc
		ch <- v2WeatherRefreshTimestampDesc
		ch <- v2WeatherRefreshDurationDesc
		ch <- v2WeatherCacheTimestampDesc
	}

	// Homecoach meta descriptors
	if c.enableHomecoach {
		ch <- v2HomecoachUpDesc
		ch <- v2HomecoachRefreshIntervalDesc
		ch <- v2HomecoachRefreshTimestampDesc
		ch <- v2HomecoachRefreshDurationDesc
		ch <- v2HomecoachCacheTimestampDesc
	}
}

func (c *UnifiedCollectorV2) Collect(ch chan<- prometheus.Metric) {
	now := c.clock()

	if c.enableWeather {
		if now.Sub(c.weatherLastRefresh) >= c.refreshInterval {
			go c.refreshWeather(now)
		}
		c.collectWeatherMetaV2(ch)
		c.collectWeatherV2(ch)
	}

	if c.enableHomecoach {
		if now.Sub(c.homecoachLastRefresh) >= c.refreshInterval {
			go c.refreshHomecoach(now)
		}
		c.collectHomecoachMetaV2(ch)
		c.collectHomecoachV2(ch)
	}
}

func (c *UnifiedCollectorV2) refreshWeather(now time.Time) {
	c.log.Debugf("V2: refreshing weather data")

	start := c.clock()
	defer func() {
		c.weatherLock.Lock()
		c.weatherLastRefreshDuration = c.clock().Sub(start)
		c.weatherLock.Unlock()
	}()

	data, err := c.weatherReader()

	c.weatherLock.Lock()
	c.weatherLastRefresh = now
	c.weatherLastRefreshError = err
	if err != nil {
		c.weatherLock.Unlock()
		c.log.Errorf("V2 Weather: error during refresh: %s", err)
		return
	}
	c.weatherCachedData = data
	c.weatherLock.Unlock()
}

func (c *UnifiedCollectorV2) refreshHomecoach(now time.Time) {
	c.log.Debugf("V2: refreshing Homecoach data")

	start := c.clock()
	defer func() {
		c.homecoachLock.Lock()
		c.homecoachLastRefreshDuration = c.clock().Sub(start)
		c.homecoachLock.Unlock()
	}()

	data, err := c.homecoachReader()

	c.homecoachLock.Lock()
	c.homecoachLastRefresh = now
	c.homecoachLastRefreshError = err
	if err != nil {
		c.homecoachLock.Unlock()
		c.log.Errorf("V2 Homecoach: error during refresh: %s", err)
		return
	}
	c.homecoachCachedData = data
	c.homecoachLock.Unlock()
}

func (c *UnifiedCollectorV2) collectWeatherMetaV2(ch chan<- prometheus.Metric) {
	c.weatherLock.RLock()
	defer c.weatherLock.RUnlock()

	upValue := 1.0
	if c.weatherLastRefresh.IsZero() || c.weatherLastRefreshError != nil {
		upValue = 0
	}

	sendMetric(c.log, ch, v2WeatherUpDesc, prometheus.GaugeValue, upValue)
	sendMetric(c.log, ch, v2WeatherRefreshIntervalDesc, prometheus.GaugeValue, c.refreshInterval.Seconds())
	sendMetric(c.log, ch, v2WeatherRefreshTimestampDesc, prometheus.GaugeValue, convertTime(c.weatherLastRefresh))
	sendMetric(c.log, ch, v2WeatherRefreshDurationDesc, prometheus.GaugeValue, c.weatherLastRefreshDuration.Seconds())
	sendMetric(c.log, ch, v2WeatherCacheTimestampDesc, prometheus.GaugeValue, convertTime(c.weatherLastRefresh))
}

func (c *UnifiedCollectorV2) collectHomecoachMetaV2(ch chan<- prometheus.Metric) {
	c.homecoachLock.RLock()
	defer c.homecoachLock.RUnlock()

	upValue := 1.0
	if c.homecoachLastRefresh.IsZero() || c.homecoachLastRefreshError != nil {
		upValue = 0
	}

	sendMetric(c.log, ch, v2HomecoachUpDesc, prometheus.GaugeValue, upValue)
	sendMetric(c.log, ch, v2HomecoachRefreshIntervalDesc, prometheus.GaugeValue, c.refreshInterval.Seconds())
	sendMetric(c.log, ch, v2HomecoachRefreshTimestampDesc, prometheus.GaugeValue, convertTime(c.homecoachLastRefresh))
	sendMetric(c.log, ch, v2HomecoachRefreshDurationDesc, prometheus.GaugeValue, c.homecoachLastRefreshDuration.Seconds())
	sendMetric(c.log, ch, v2HomecoachCacheTimestampDesc, prometheus.GaugeValue, convertTime(c.homecoachLastRefresh))
}

func (c *UnifiedCollectorV2) collectWeatherV2(ch chan<- prometheus.Metric) {
	c.weatherLock.RLock()
	defer c.weatherLock.RUnlock()

	if c.weatherCachedData == nil {
		return
	}

	for _, dev := range c.weatherCachedData.Devices() {
		homeName := dev.HomeName
		stationName := dev.StationName //nolint: staticcheck

		c.collectWeatherDeviceV2(ch, dev, stationName, homeName)
		for _, module := range dev.LinkedModules {
			c.collectWeatherDeviceV2(ch, module, stationName, homeName)
		}
	}
}

func (c *UnifiedCollectorV2) collectWeatherDeviceV2(ch chan<- prometheus.Metric, device *netatmo.Device, stationName, homeName string) {
	moduleName := device.ModuleName
	if moduleName == "" {
		moduleName = "id-" + device.ID
	}

	data := device.DashboardData
	if data.LastMeasure == nil {
		return
	}

	date := time.Unix(*data.LastMeasure, 0)
	dataAge := c.clock().Sub(date)
	if dataAge > c.staleThreshold {
		c.log.Debugf("V2: Data stale for %s: %s > %s", moduleName, dataAge, c.staleThreshold)
		return
	}

	// Unified labels: device_class, device_id, home, module, station
	labels := []string{"weather", device.ID, homeName, moduleName, stationName}

	sendMetric(c.log, ch, v2UpdatedDesc, prometheus.GaugeValue, float64(date.UTC().Unix()), labels...)

	if data.Temperature != nil {
		sendMetric(c.log, ch, v2TempDesc, prometheus.GaugeValue, float64(*data.Temperature), labels...)
	}
	if data.Humidity != nil {
		sendMetric(c.log, ch, v2HumidityDesc, prometheus.GaugeValue, float64(*data.Humidity), labels...)
	}
	if data.CO2 != nil {
		sendMetric(c.log, ch, v2CO2Desc, prometheus.GaugeValue, float64(*data.CO2), labels...)
	}
	if data.Noise != nil {
		sendMetric(c.log, ch, v2NoiseDesc, prometheus.GaugeValue, float64(*data.Noise), labels...)
	}
	if data.Pressure != nil {
		sendMetric(c.log, ch, v2PressureDesc, prometheus.GaugeValue, float64(*data.Pressure), labels...)
	}
	if data.WindStrength != nil {
		sendMetric(c.log, ch, v2WindStrengthDesc, prometheus.GaugeValue, float64(*data.WindStrength), labels...)
	}
	if data.WindAngle != nil {
		sendMetric(c.log, ch, v2WindDirectionDesc, prometheus.GaugeValue, float64(*data.WindAngle), labels...)
	}
	if data.Rain != nil {
		sendMetric(c.log, ch, v2RainDesc, prometheus.GaugeValue, float64(*data.Rain), labels...)
	}
	if device.BatteryPercent != nil {
		sendMetric(c.log, ch, v2BatteryDesc, prometheus.GaugeValue, float64(*device.BatteryPercent), labels...)
	}
	if device.WifiStatus != nil {
		sendMetric(c.log, ch, v2WifiDesc, prometheus.GaugeValue, float64(*device.WifiStatus), labels...)
	}
	if device.RFStatus != nil {
		sendMetric(c.log, ch, v2RFDesc, prometheus.GaugeValue, float64(*device.RFStatus), labels...)
	}
}

func (c *UnifiedCollectorV2) collectHomecoachV2(ch chan<- prometheus.Metric) {
	c.homecoachLock.RLock()
	defer c.homecoachLock.RUnlock()

	if c.homecoachCachedData == nil {
		return
	}

	for _, device := range c.homecoachCachedData.Body.Devices {
		// Unified labels: device_class, device_id, home, module, station
		labels := []string{"homecoach", device.ID, "", "", device.StationName}
		dd := device.DashboardData

		sendMetric(c.log, ch, v2UpdatedDesc, prometheus.GaugeValue, float64(dd.TimeUTC), labels...)
		sendMetric(c.log, ch, v2TempDesc, prometheus.GaugeValue, float64(dd.Temperature), labels...)
		sendMetric(c.log, ch, v2HumidityDesc, prometheus.GaugeValue, float64(dd.Humidity), labels...)
		sendMetric(c.log, ch, v2CO2Desc, prometheus.GaugeValue, float64(dd.CO2), labels...)
		sendMetric(c.log, ch, v2NoiseDesc, prometheus.GaugeValue, float64(dd.Noise), labels...)
		sendMetric(c.log, ch, v2PressureDesc, prometheus.GaugeValue, float64(dd.Pressure), labels...)
		sendMetric(c.log, ch, v2HealthIndexDesc, prometheus.GaugeValue, float64(dd.HealthIndex), labels...)
		sendMetric(c.log, ch, v2WifiDesc, prometheus.GaugeValue, float64(device.WifiStatus), labels...)
	}
}

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
