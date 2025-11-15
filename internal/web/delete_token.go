package web

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/exzz/netatmo-api-go"
	"github.com/sirupsen/logrus"
)

// DeleteTokenHandler creates a handler that deletes the token file.
// This ensures that on restart, no old token is loaded.
func DeleteTokenHandler(ctx context.Context, client *netatmo.Client, tokenFile string, log logrus.FieldLogger) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		// Only allow POST to prevent accidental deletion via GET
		if r.Method != http.MethodPost {
			http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Delete the token file if it exists
		if tokenFile != "" {
			err := os.Remove(tokenFile)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Errorf("Failed to delete token file %s: %s", tokenFile, err)
				http.Error(wr, "Failed to delete token file", http.StatusInternalServerError)
				return
			}
			log.Infof("Token file deleted or already absent: %s", tokenFile)
		}

		// Clear the token in memory (so user sees auth form immediately)
		client.InitWithToken(ctx, nil)
		log.Info("Token cleared from memory. Please re-authenticate to create a new token.")

		// Redirect back to home page
		http.Redirect(wr, r, "/", http.StatusFound)
	}
}
