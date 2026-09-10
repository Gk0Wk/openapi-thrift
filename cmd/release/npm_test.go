package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This exercises the real pinned npm client against a loopback-only registry.
// Run with `npm run test:release`; ordinary Go consumers do not require Node.
func TestNPMPublishPreservesFirstResponseWithoutRetry(t *testing.T) {
	npmCLI := os.Getenv("npm_execpath")
	if npmCLI == "" || os.Getenv("npm_lifecycle_event") != "test:release" {
		t.Skip("run npm run test:release to exercise the npm publishing client")
	}
	if filepath.Base(npmCLI) != "npm-cli.js" {
		t.Fatal("test:release must run through npm, not another package manager")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		firstStatus int
		retries     string
		wantPuts    int32
		wantError   string
	}{
		{"accepted", http.StatusCreated, "0", 1, ""},
		{"unauthorized", http.StatusUnauthorized, "0", 1, "E401"},
		{"accepted_but_response_failed", http.StatusServiceUnavailable, "0", 1, "E503"},
		{"control_default_retry_masks_first_response", http.StatusServiceUnavailable, "", 2, "E401"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var puts atomic.Int32
			registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/@release-test/publish-probe" {
					t.Errorf("unexpected registry request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected path", http.StatusBadRequest)
					return
				}
				if r.Method == http.MethodGet {
					_, _ = io.WriteString(w, `{"name":"@release-test/publish-probe","versions":{}}`)
					return
				}
				if r.Method != http.MethodPut {
					t.Errorf("unexpected registry method: %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				_, _ = io.Copy(io.Discard, r.Body)
				if r.Header.Get("Authorization") != "Bearer synthetic-publish-test" {
					t.Error("unexpected test authentication")
				}
				status := tc.firstStatus
				if puts.Add(1) > 1 {
					status = http.StatusUnauthorized
				}
				w.WriteHeader(status)
				switch status {
				case http.StatusCreated:
					_, _ = io.WriteString(w, `{"ok":true}`)
				case http.StatusServiceUnavailable:
					_, _ = io.WriteString(w, `{"error":"simulated failure after accepting package"}`)
				default:
					_, _ = io.WriteString(w, `{"error":"simulated invalid publish token"}`)
				}
			}))
			defer registry.Close()
			dir := t.TempDir()
			manifest := `{"name":"@release-test/publish-probe","version":"0.0.0","files":[],"license":"MIT"}`
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, "test.npmrc")
			auth := fmt.Sprintf("//%s/:_authToken=synthetic-publish-test\n", strings.TrimPrefix(registry.URL, "http://"))
			if err := os.WriteFile(config, []byte(auth), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, node, npmCLI, "publish", dir,
				"--ignore-scripts", "--access=public", "--provenance=false", "--tag=test",
				"--update-notifier=false",
				"--registry="+registry.URL+"/", "--userconfig="+config,
				"--globalconfig="+filepath.Join(dir, "empty-global.npmrc"),
				"--cache="+filepath.Join(dir, "cache"),
				"--fetch-retry-mintimeout=1", "--fetch-retry-maxtimeout=1")
			cmd.Dir = dir
			// Do not inherit real npm, OIDC, proxy or CI credentials/configuration.
			for _, key := range []string{"PATH", "SystemRoot", "WINDIR", "TEMP", "TMP", "HOME", "USERPROFILE"} {
				if value, ok := os.LookupEnv(key); ok {
					cmd.Env = append(cmd.Env, key+"="+value)
				}
			}
			if tc.retries != "" {
				cmd.Env = append(cmd.Env, "NPM_CONFIG_FETCH_RETRIES="+tc.retries)
			}
			output, err := cmd.CombinedOutput()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("publish failed: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(string(output), tc.wantError) {
				t.Fatalf("expected %s, got %v\n%s", tc.wantError, err, output)
			}
			if got := puts.Load(); got != tc.wantPuts {
				t.Fatalf("PUT count = %d, want %d\n%s", got, tc.wantPuts, output)
			}
		})
	}
}
