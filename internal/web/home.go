package web

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	_ "embed"
)

const netatmoDevSite = "https://dev.netatmo.com/apps/"

//go:embed home.html
var homeHtml string

type homeContext struct {
	Valid          bool
	Token          *oauth2.Token
	NetAtmoDevSite string
}

// HomeHandler produces a simple website showing the exporter's status in a human-readable form.
// It provides links to other information and help for authentication as well.
func HomeHandler(tokenFunc func() (*oauth2.Token, error), log interface{ Warnf(string, ...interface{}) }) http.Handler {
	homeTemplate, err := template.New("home.html").Funcs(map[string]any{
		"remaining": remaining,
	}).Parse(homeHtml)
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		token, err := tokenFunc()
		if err != nil {
			// Log that token retrieval failed. We cannot distinguish between:
			// - No token was ever set (expected)
			// - Token was deleted (expected)
			// - Token is expired and refresh failed (unexpected)
			// API limitation: the underlying netatmo.Client returns nil token + error in all these cases
			// Without API changes to return different error types, we log all cases equally.
			log.Warnf("Token invalid or no token found: %v", err)
			token = nil
		}

		context := homeContext{
			Valid:          token != nil && token.Valid(),
			Token:          token,
			NetAtmoDevSite: netatmoDevSite,
		}

		wr.Header().Set("Content-Type", "text/html")
		if err := homeTemplate.Execute(wr, context); err != nil {
			http.Error(wr, fmt.Sprintf("Error executing template: %s", err), http.StatusInternalServerError)
		}
	})
}

func remaining(t time.Time) time.Duration {
	return time.Until(t)
}
