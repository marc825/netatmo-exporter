package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/exzz/netatmo-api-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

func AuthorizeHandler(externalURL string, client *netatmo.Client, enableWeather, enableHomecoach bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirectURL := externalURL + "/auth/callback"
		baseAuthURL := client.AuthCodeURL(redirectURL, "definitelyrandom")

		// Build the final auth URL with dynamic scopes
		authURL := BuildAuthURL(baseAuthURL, enableWeather, enableHomecoach)

		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func CallbackHandler(ctx context.Context, client *netatmo.Client, log logrus.FieldLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		if err := doCallback(ctx, client, values); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error processing code: %s", err)
			return
		}

		log.Info("Successfully authenticated and created new token via OAuth")
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func doCallback(ctx context.Context, client *netatmo.Client, query url.Values) error {
	if err := query.Get("error"); err != "" {
		return errors.New("user did not accept")
	}

	state := query.Get("state")
	code := query.Get("code")

	return client.Exchange(ctx, code, state)
}

func SetTokenHandler(ctx context.Context, client *netatmo.Client, log logrus.FieldLogger) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		refreshToken := r.FormValue("refresh_token")
		if refreshToken == "" {
			http.Error(wr, "The refresh token can not be empty. Please go back.", http.StatusBadRequest)
			return
		}

		token := &oauth2.Token{
			RefreshToken: refreshToken,
		}
		client.InitWithToken(ctx, token)

		log.Info("Successfully set new token manually via refresh token")
		http.Redirect(wr, r, "/", http.StatusFound)
	}
}
