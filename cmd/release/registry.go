package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const npmRegistry = "https://registry.npmjs.org"
const npmPackage = "@sttot/openapi-thrift"
const maxNPMBytes = 16 << 20

func verifyNPM(tag, archive, output string) error {
	version := strings.TrimPrefix(tag, "v")
	if !strings.HasPrefix(tag, "v") {
		return fmt.Errorf("verify-npm requires an explicit v-prefixed tag")
	}
	if err := checkVersion(version, tag); err != nil {
		return err
	}
	expected, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	if len(expected) == 0 || len(expected) > maxNPMBytes {
		return fmt.Errorf("expected npm tarball is empty or exceeds 16 MiB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return fmt.Errorf("registry verification does not follow redirects")
	}}
	wait := func(ctx context.Context) error {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	body, err := readPublishedNPM(ctx, client, npmRegistry, version, expected, wait)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	path := filepath.Join(output, "sttot-openapi-thrift-"+version+".tgz")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Printf("Verified registry integrity and exact npm bytes: %s\n", path)
	return nil
}

// Read the explicit version endpoint, not npm's mutable package-version index.
// Waiting is limited to read visibility failures; identity errors fail at once.
func readPublishedNPM(ctx context.Context, client *http.Client, registry, version string, expected []byte, wait func(context.Context) error) ([]byte, error) {
	digest := sha512.Sum512(expected)
	wantIntegrity := "sha512-" + base64.StdEncoding.EncodeToString(digest[:])
	metadataURL := registry + "/@sttot%2fopenapi-thrift/" + version
	wantTarball := registry + "/@sttot/openapi-thrift/-/openapi-thrift-" + version + ".tgz"
	var lastStatus int
	for attempt := 0; attempt < 12; attempt++ {
		if attempt > 0 {
			if err := wait(ctx); err != nil {
				return nil, err
			}
		}
		body, status, err := registryGET(ctx, client, metadataURL, 1<<20)
		if err != nil {
			return nil, err
		}
		lastStatus = status
		if retryableRegistryStatus(status) {
			continue
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("npm version metadata returned HTTP %d", status)
		}
		var meta struct {
			Name, Version string
			Dist          struct{ Integrity, Tarball string }
		}
		if err := json.Unmarshal(body, &meta); err != nil {
			return nil, fmt.Errorf("invalid npm version metadata: %w", err)
		}
		if meta.Name != npmPackage || meta.Version != version || meta.Dist.Integrity != wantIntegrity || meta.Dist.Tarball != wantTarball {
			return nil, fmt.Errorf("npm package identity, integrity or tarball location differs from the tested release")
		}
		body, status, err = registryGET(ctx, client, wantTarball, maxNPMBytes)
		if err != nil {
			return nil, err
		}
		lastStatus = status
		if retryableRegistryStatus(status) {
			continue
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("npm tarball returned HTTP %d", status)
		}
		if !bytes.Equal(body, expected) {
			return nil, fmt.Errorf("published npm bytes differ from the tested release")
		}
		return body, nil
	}
	return nil, fmt.Errorf("npm release is not readable after 12 attempts (last HTTP %d)", lastStatus)
}

func retryableRegistryStatus(status int) bool {
	return status == 404 || status == 408 || status == 429 || status >= 500 && status < 600
}

func registryGET(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	response, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if int64(len(body)) > limit {
		return nil, response.StatusCode, fmt.Errorf("registry response exceeds %d bytes", limit)
	}
	return body, response.StatusCode, nil
}
