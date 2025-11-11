package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/exzz/netatmo-api-go"
	"github.com/sirupsen/logrus"
	"github.com/xperimental/netatmo-exporter/v2/internal/collector"
	"golang.org/x/oauth2"
// DebugNetatmoHandler creates a handler that displays both weather and homecoach data
func DebugNetatmoHandler(log logrus.FieldLogger, weatherReadFunc func() (*netatmo.DeviceCollection, error), homecoachReadFunc func() (*collector.HomeCoachResponse, error)) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		// only allow GET
		if r.Method != http.MethodGet {
			http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := struct {
			Weather   interface{} `json:"weather"`
			HomeCoach interface{} `json:"homecoach"`
			Error     *string     `json:"error,omitempty"`
		}{}

		var weatherErr, homecoachErr error

		// Weather Data
		weatherData, weatherErr := weatherReadFunc()
		if weatherErr != nil {
			errMsg := fmt.Sprintf("Error retrieving weather data: %s", weatherErr)
			response.Weather = map[string]string{"error": errMsg}
			log.Warnf("Debug handler: %s", errMsg)
		} else if weatherData != nil {
			// extract only the Devices
			response.Weather = map[string]interface{}{
				"devices": weatherData.Devices(),
			}
		} else {
			response.Weather = map[string]interface{}{
				"devices": []interface{}{},
			}
		}

		// HomeCoach Data
		homecoachData, homecoachErr := homecoachReadFunc()
		if homecoachErr != nil {
			errMsg := fmt.Sprintf("Error retrieving homecoach data: %s", homecoachErr)
			response.HomeCoach = map[string]string{"error": errMsg}
			log.Warnf("Debug handler: %s", errMsg)
		} else if homecoachData != nil && homecoachData.Body.Devices != nil {
			// extract only the devices
			response.HomeCoach = map[string]interface{}{
				"devices": homecoachData.Body.Devices,
			}
		} else {
			response.HomeCoach = map[string]interface{}{
				"devices": []interface{}{},
			}
		}

		// http status
		statusCode := http.StatusOK
		if weatherErr != nil && homecoachErr != nil {
			overallErr := "Both weather and homecoach data retrieval failed"
			response.Error = &overallErr
			statusCode = http.StatusBadGateway
		} else if weatherErr != nil || homecoachErr != nil {
			statusCode = http.StatusPartialContent
		}

		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(statusCode)
		enc := json.NewEncoder(wr)
		enc.SetIndent("", "  ")
		if err := enc.Encode(response); err != nil {
			log.Errorf("Can not encode debug response: %s", err)
			return
		}
	})
}

// DebugDataHandler creates a handler which returns the raw data retrieved from Netatmo API
func DebugDataHandler(log logrus.FieldLogger, readFunc func() (*netatmo.DeviceCollection, error)) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		// only allow GET
		if r.Method != http.MethodGet {
			http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		devices, err := readFunc()
		if err != nil {
			http.Error(wr, fmt.Sprintf("Error retrieving data: %s", err), http.StatusBadGateway)
			return
		}

		wr.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(wr)
		enc.SetIndent("", "  ")
		if err := enc.Encode(devices); err != nil {
			log.Errorf("Can not encode data debug response: %s", err)
			return
		}
	})
}

// DebugTokenHandler creates a handler which returns information about the currently-used token
func DebugTokenHandler(log logrus.FieldLogger, tokenFunc func() (*oauth2.Token, error)) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		// only allow GET
		if r.Method != http.MethodGet {
			http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token, err := tokenFunc()
		switch {
		case err == netatmo.ErrNotAuthenticated:
		case err != nil:
			http.Error(wr, fmt.Sprintf("Error retrieving token: %s", err), http.StatusInternalServerError)
			return
		default:
		}

		if token == nil {
			http.Error(wr, "No token available.", http.StatusNotFound)
			return
		}

		data := struct {
			IsValid         bool      `json:"isValid"`
			HasAccessToken  bool      `json:"hasAccessToken"`
			HasRefreshToken bool      `json:"hasRefreshToken"`
			Expiry          time.Time `json:"expiry"`
		}{
			IsValid:         token.Valid(),
			HasAccessToken:  token.AccessToken != "",
			HasRefreshToken: token.RefreshToken != "",
			Expiry:          token.Expiry,
		}

		wr.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(wr)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			log.Errorf("Can not encode token debug response: %s", err)
			return
		}
	})
}
