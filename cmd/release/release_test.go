package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseVersionMustMatchSafeImmutableTag(t *testing.T) {
	for _, version := range []string{"0.3.0-rc.1", "1.0.0", "2.3.4-beta.2"} {
		if err := checkVersion(version, "v"+version); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]string{{"0.3.0-rc.1", "v0.3.0"}, {"01.2.3", ""}, {"1.2.3-rc.01", ""}, {"1.2.3\nnext=bad", ""}, {"../../1.2.3", ""}, {"1.2.3+mutable", ""}, {"1.2", ""}, {"1.2.3-", ""}} {
		if err := checkVersion(pair[0], pair[1]); err == nil {
			t.Fatalf("unsafe version accepted: %q", pair)
		}
	}
}

func TestNativeArchivesPreserveBytesAndRefuseOverwrite(t *testing.T) {
	for _, extension := range []string{".zip", ".tar.gz"} {
		t.Run(extension, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "release"+extension)
			files := []archiveFile{{"openapi-thrift", []byte("binary"), 0755}, {"LICENSE", []byte("license"), 0644}, {"release.json", []byte("{}"), 0644}}
			if err := writeArchive(path, files); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := readArchive(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range files {
				if !bytes.Equal(got[want.name].body, want.body) || got[want.name].mode.Perm() != want.mode {
					t.Fatalf("lost archive content or permissions: %s", want.name)
				}
			}
			if err := writeArchive(path, files); err == nil {
				t.Fatal("existing immutable artifact overwritten")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("existing artifact changed")
			}
		})
	}
}

func TestArchiveRejectsUnexpectedDuplicateAndSymlinkEntries(t *testing.T) {
	for name, files := range map[string][]archiveFile{
		"traversal": {{"../escape", []byte("bad"), 0644}},
		"duplicate": {{"LICENSE", nil, 0644}, {"LICENSE", nil, 0644}},
		"symlink":   {{"openapi-thrift", []byte("outside"), os.ModeSymlink | 0755}},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "unsafe.zip")
			if err := writeArchive(path, files); err != nil {
				t.Fatal(err)
			}
			if _, err := readArchive(path); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestNativeSmokeRejectsCorruptedBinaryBeforeExecution(t *testing.T) {
	name := "openapi-thrift"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	meta := metadata{Version: "0.3.0-rc.1", Commit: strings.Repeat("a", 40), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, BinarySHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("original")))}
	manifest, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "corrupt.zip")
	if err := writeArchive(path, []archiveFile{{name, []byte("corrupt"), 0755}, {"LICENSE", []byte("MIT"), 0644}, {"release.json", manifest, 0644}}); err != nil {
		t.Fatal(err)
	}
	if err := smokeArchive(path); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("missing integrity failure: %v", err)
	}
}
