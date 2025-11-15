package web

import (
	"net/url"
	"strings"
)

// BuildAuthURL builds the authorization URL with dynamic scopes based on enabled collectors.
// This is a workaround for the netatmo-api-go library which hardcodes scopes to "read_station".
// TODO: This can be removed once netatmo-api-go supports dynamic scopes natively.
func BuildAuthURL(baseAuthURL string, enableWeather, enableHomecoach bool) string {
	scopes := buildScopes(enableWeather, enableHomecoach)
	return replaceScopes(baseAuthURL, scopes)
}

// buildScopes creates the list of OAuth scopes based on enabled collectors.
func buildScopes(enableWeather, enableHomecoach bool) []string {
	var scopes []string

	if enableWeather {
		scopes = append(scopes, "read_station")
	}

	if enableHomecoach {
		scopes = append(scopes, "read_homecoach")
	}

	return scopes
}

// replaceScopes replaces the existing scope parameter in the authorization URL to prevent duplication of the 'read_station' scope.
func replaceScopes(authURL string, scopes []string) string {
	parsedURL, err := url.Parse(authURL)
	if err != nil {
		// This should never happen. This is here as a defensive check.
		return authURL
	}

	query := parsedURL.Query()

	// Replace the scope parameter
	query.Set("scope", strings.Join(scopes, " "))

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}
