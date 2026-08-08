// Command gdrive-auth performs the one-time OAuth user flow to obtain a Google
// Drive refresh token for this app's payment-proof uploads. It starts a loopback
// HTTP server, opens the consent screen in the browser, exchanges the returned
// code for a refresh token (PKCE), and prints GOOGLE_DRIVE_REFRESH_TOKEN.
//
// Usage:
//
//	cd server
//	GOOGLE_DRIVE_CLIENT_ID=<id> GOOGLE_DRIVE_CLIENT_SECRET=<secret> go run ./cmd/gdrive-auth
//
// Prerequisites: an OAuth Client ID of type "Desktop app" in Google Cloud
// Console with the Google Drive API enabled, plus the app listed as a test user
// on the OAuth consent screen.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gdrive "google.golang.org/api/drive/v3"
)

const oauthState = "gdrive-auth-state"

func main() {
	clientID := os.Getenv("GOOGLE_DRIVE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_DRIVE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("set GOOGLE_DRIVE_CLIENT_ID and GOOGLE_DRIVE_CLIENT_SECRET")
	}

	// Loopback redirect: Google requires localhost callback for desktop apps.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to start loopback listener: %v", err)
	}
	defer listener.Close()
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/oauth2callback", listener.Addr().(*net.TCPAddr).Port)

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       []string{gdrive.DriveFileScope},
	}

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != oauthState {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "no code in response", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Authorization successful! You can close this tab and return to the terminal.")
		codeCh <- code
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Close()

	// PKCE is required by Google for public (desktop) clients.
	verifier := oauth2.GenerateVerifier()
	authURL := conf.AuthCodeURL(oauthState, oauth2.S256ChallengeOption(verifier))
	fmt.Printf("\nOpen this URL in your browser and sign in with the Google account that owns the Drive folder:\n\n%s\n\n", authURL)
	openBrowser(authURL)

	code := <-codeCh

	tok, err := conf.Exchange(context.Background(), code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Fatalf("token exchange failed: %v", err)
	}

	fmt.Println("\nAdd this to server/.env:")
	fmt.Printf("GOOGLE_DRIVE_REFRESH_TOKEN=%s\n", tok.RefreshToken)
}

// openBrowser best-effort launches the default browser; silent failure is fine.
func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser automatically, copy the URL above: %v", err)
	}
}
