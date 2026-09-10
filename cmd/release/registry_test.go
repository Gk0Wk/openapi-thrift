package main

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryReadbackHandlesVisibilityAndFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, fault, wantError string
		wantReads, wantWaits   int
	}{
		{"visible", "", "", 1, 0},
		{"metadata_propagation", "metadata404", "", 2, 1},
		{"tarball_propagation", "tarball404", "", 2, 1},
		{"server_failure", "metadata503", "", 2, 1},
		{"unauthorized", "unauthorized", "HTTP 401", 1, 0},
		{"wrong_version", "version", "identity", 1, 0},
		{"wrong_integrity", "integrity", "integrity", 1, 0},
		{"foreign_tarball", "location", "location", 1, 0},
		{"corrupt_tarball", "bytes", "bytes differ", 1, 0},
		{"never_visible", "never", "after 12 attempts", 12, 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := []byte("exact tested npm archive")
			digest := sha512.Sum512(expected)
			reads, waits := 0, 0
			var registry *httptest.Server
			registry = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Cache-Control") != "no-cache" || r.Header.Get("Authorization") != "" {
					t.Error("readback must use fresh, unauthenticated GET requests")
				}
				switch r.URL.Path {
				case "/@sttot/openapi-thrift/0.3.0":
					reads++
					if tc.fault == "never" || tc.fault == "metadata404" && reads == 1 {
						w.WriteHeader(404)
						return
					}
					if tc.fault == "metadata503" && reads == 1 {
						w.WriteHeader(503)
						return
					}
					if tc.fault == "unauthorized" {
						w.WriteHeader(401)
						return
					}
					version := "0.3.0"
					integrity := "sha512-" + base64.StdEncoding.EncodeToString(digest[:])
					tarball := registry.URL + "/@sttot/openapi-thrift/-/openapi-thrift-0.3.0.tgz"
					switch tc.fault {
					case "version":
						version = "0.2.0"
					case "integrity":
						integrity = "sha512-wrong"
					case "location":
						tarball = "https://example.invalid/untrusted.tgz"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"name": npmPackage, "version": version, "dist": map[string]string{"integrity": integrity, "tarball": tarball}})
				case "/@sttot/openapi-thrift/-/openapi-thrift-0.3.0.tgz":
					if tc.fault == "tarball404" && reads == 1 {
						w.WriteHeader(404)
						return
					}
					if tc.fault == "bytes" {
						_, _ = io.WriteString(w, "corrupt")
						return
					}
					_, _ = w.Write(expected)
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
					w.WriteHeader(400)
				}
			}))
			defer registry.Close()
			_, err := readPublishedNPM(t.Context(), registry.Client(), registry.URL, "0.3.0", expected, func(context.Context) error { waits++; return nil })
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			if reads != tc.wantReads || waits != tc.wantWaits {
				t.Fatalf("reads/waits = %d/%d, want %d/%d", reads, waits, tc.wantReads, tc.wantWaits)
			}
		})
	}
}

func TestRegistryReadbackStopsWhenCanceled(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer registry.Close()
	_, err := readPublishedNPM(t.Context(), registry.Client(), registry.URL, "0.3.0", []byte("expected"), func(context.Context) error { return context.Canceled })
	if err != context.Canceled {
		t.Fatalf("got %v, want cancellation", err)
	}
}

func TestRegistryResponseSizeIsBounded(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "oversized") }))
	defer registry.Close()
	if _, _, err := registryGET(t.Context(), registry.Client(), registry.URL, 3); err == nil {
		t.Fatal("oversized response accepted")
	}
}
