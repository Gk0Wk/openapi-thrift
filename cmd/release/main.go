// release builds and verifies the artifacts shipped by the release workflow.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const modulePath = "github.com/Gk0Wk/openapi-thrift"

type metadata struct {
	Version      string `json:"version"`
	Commit       string `json:"commit"`
	GoVersion    string `json:"go_version"`
	GOOS         string `json:"goos,omitempty"`
	GOARCH       string `json:"goarch,omitempty"`
	BinarySHA256 string `json:"binary_sha256,omitempty"`
	Dirty        bool   `json:"dirty"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: release metadata|build|smoke|smoke-go [flags]")
	}
	flags := flag.NewFlagSet("release "+args[0], flag.ContinueOnError)
	tag := flags.String("tag", "", "Release tag, which must match package.json")
	out := flags.String("output", ".tmp/release", "Artifact directory")
	archive := flags.String("archive", "", "Native release archive to install and test")
	moduleVersion := flags.String("module-version", "", "Published Go version; otherwise test local source")
	target := flags.String("target", "", "Expected native GOOS/GOARCH")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional argument")
	}
	if args[0] == "smoke" {
		return smokeArchive(*archive)
	}
	if args[0] == "smoke-go" {
		return smokeGo(*moduleVersion)
	}
	meta, err := readMetadata(*tag)
	if err != nil {
		return err
	}
	switch args[0] {
	case "metadata":
		channel := "latest"
		prerelease := strings.Contains(meta.Version, "-")
		if prerelease {
			channel = "next"
		}
		fmt.Printf("version=%s\ntag=v%s\ncommit=%s\nnpm_tag=%s\nprerelease=%t\n", meta.Version, meta.Version, meta.Commit, channel, prerelease)
		return nil
	case "build":
		if meta.Dirty {
			return errors.New("release artifacts require a clean committed checkout")
		}
		if *target != "" && *target != runtime.GOOS+"/"+runtime.GOARCH {
			return fmt.Errorf("runner target differs: got %s/%s, expected %s", runtime.GOOS, runtime.GOARCH, *target)
		}
		return build(meta, *out)
	default:
		return fmt.Errorf("unknown release command %q", args[0])
	}
}

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

func checkVersion(version, tag string) error {
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("invalid release version %q", version)
	}
	if _, suffix, ok := strings.Cut(version, "-"); ok {
		for _, part := range strings.Split(suffix, ".") {
			if len(part) > 1 && part[0] == '0' && strings.Trim(part, "0123456789") == "" {
				return fmt.Errorf("numeric prerelease identifier has a leading zero: %q", part)
			}
		}
	}
	if tag != "" && tag != "v"+version {
		return fmt.Errorf("release tag %q differs from package version %q", tag, version)
	}
	return nil
}

func readMetadata(tag string) (metadata, error) {
	var manifest struct{ Name, Version string }
	body, err := os.ReadFile("package.json")
	if err != nil {
		return metadata{}, err
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return metadata{}, err
	}
	if manifest.Name != "@sttot/openapi-thrift" {
		return metadata{}, errors.New("unexpected npm package name")
	}
	if err := checkVersion(manifest.Version, tag); err != nil {
		return metadata{}, err
	}
	if _, err := os.Stat(filepath.Join("docs", "releases", "v"+manifest.Version+".md")); err != nil {
		return metadata{}, fmt.Errorf("version-specific release notes are required: %w", err)
	}
	goMod, err := os.ReadFile("go.mod")
	if err != nil {
		return metadata{}, err
	}
	if !strings.HasPrefix(string(goMod), "module "+modulePath+"\n") {
		return metadata{}, errors.New("unexpected Go module path")
	}
	if !strings.Contains(string(goMod), "\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n") {
		return metadata{}, fmt.Errorf("release build must use go.mod's exact Go version; got %s", runtime.Version())
	}
	commit, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		return metadata{}, err
	}
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit) {
		return metadata{}, errors.New("invalid source revision")
	}
	status, err := commandOutput("git", "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return metadata{}, err
	}
	if tag != "" && status != "" {
		return metadata{}, errors.New("release tag requires a clean checkout")
	}
	if tag != "" {
		resolved, err := commandOutput("git", "rev-parse", "refs/tags/"+tag+"^{commit}")
		if err != nil {
			return metadata{}, err
		}
		if resolved != commit {
			return metadata{}, errors.New("release tag points to another source revision")
		}
	}
	return metadata{Version: manifest.Version, Commit: commit, GoVersion: runtime.Version(), Dirty: status != ""}, nil
}

func commandOutput(name string, args ...string) (string, error) {
	body, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w\n%s", name, err, body)
	}
	return strings.TrimSpace(string(body)), nil
}

func build(meta metadata, output string) error {
	if (runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return errors.New("unsupported release target")
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(output, "native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	binaryName := "openapi-thrift"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(stage, binaryName)
	ldflags := "-s -w -X " + modulePath + "/internal/cli.releaseVersion=v" + meta.Version + " -X " + modulePath + "/internal/cli.releaseCommit=" + meta.Commit
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", binaryPath, "./cmd/openapi-thrift")
	command.Env = replaceEnv(os.Environ(), "CGO_ENABLED", "0")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	binaryBody, err := os.ReadFile(binaryPath)
	if err != nil {
		return err
	}
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	meta.GOOS, meta.GOARCH = runtime.GOOS, runtime.GOARCH
	meta.BinarySHA256 = fmt.Sprintf("%x", sha256.Sum256(binaryBody))
	manifest, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	name := "openapi-thrift_" + meta.Version + "_" + meta.GOOS + "_" + meta.GOARCH + ext
	archivePath := filepath.Join(output, name)
	if err := writeArchive(archivePath, []archiveFile{{binaryName, binaryBody, 0755}, {"LICENSE", license, 0644}, {"release.json", append(manifest, '\n'), 0644}}); err != nil {
		return err
	}
	packed, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(archivePath+".sha256", []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(packed), name)), 0644); err != nil {
		return err
	}
	if err := smokeArchive(archivePath); err != nil {
		return err
	}
	fmt.Println(archivePath)
	return nil
}

func replaceEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		if !strings.EqualFold(name, key) {
			result = append(result, item)
		}
	}
	return append(result, key+"="+value)
}
