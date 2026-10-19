package server

import (
	"embed"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wayfare-labs/wayfare/asset"
	"github.com/Wayfare-labs/wayfare/runstore"
)

// uiFS holds the single-page UI. It is embedded so the binary is the whole
// deployment: no build step, no asset pipeline, no separately-served static
// directory that can drift out of sync with the API it talks to.
//
//go:embed index.html
var uiFS embed.FS

func uiHandler(store runstore.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		body, err := uiFS.ReadFile("index.html")
		if err != nil {
			http.Error(w, "ui unavailable", http.StatusInternalServerError)
			return
		}
		body = []byte(strings.Replace(string(body), "<!-- SOCIAL_METADATA -->", socialMetadata(r, store), 1))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	})
}

func socialMetadata(r *http.Request, store runstore.Store) string {
	from, to, key, ok := previewCorridor(r)
	if !ok {
		from, to, key = "USDC", "NGNC", runstore.CorridorKey("USDC", "NGNC")
	}

	title := key + " | Wayfare corridor integrity"
	description := "Recorded corridor integrity and floor data."
	if store != nil {
		record, err := store.Latest(r.Context(), key)
		if err == nil && record != nil {
			description = "Recorded " + record.Integrity + " integrity; floor loss " + record.FloorLossPct +
				"% at " + record.FloorSize + " USDC. Recorded at " + record.RecordedAt.UTC().Format(time.RFC3339Nano) + "."
		}
	}

	imageURL := &url.URL{Scheme: "https", Host: r.Host, Path: "/og/corridor.svg"}
	if r.TLS == nil && !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		imageURL.Scheme = "http"
	}
	imageURL.RawQuery = url.Values{"from": {from}, "to": {to}}.Encode()

	return `<meta property="og:type" content="website">` +
		`<meta property="og:title" content="` + html.EscapeString(title) + `">` +
		`<meta property="og:description" content="` + html.EscapeString(description) + `">` +
		`<meta property="og:image" content="` + html.EscapeString(imageURL.String()) + `">` +
		`<meta property="og:image:type" content="image/svg+xml">`
}

func previewCorridor(r *http.Request) (string, string, string, bool) {
	from := r.URL.Query().Get("from")
	if from == "" {
		from = "USDC"
	}
	to := r.URL.Query().Get("to")
	if to == "" {
		to = "NGNC"
	}
	send, sendOK := asset.Lookup(from)
	receive, receiveOK := asset.Lookup(to)
	if !sendOK || !receiveOK {
		return "", "", "", false
	}
	from, to = send.Code, receive.Code
	return from, to, runstore.CorridorKey(from, to), true
}
