package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// smokeGo builds an independent consumer, then installs and runs the CLI.
// A released version omits replace and resolves through the normal Go proxy.
func smokeGo(version string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	var goVersion string
	for _, line := range strings.Split(string(goMod), "\n") {
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	if goVersion == "" {
		return fmt.Errorf("go.mod has no Go version")
	}
	directory, err := os.MkdirTemp("", "openapi-thrift-go-consumer-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	requireVersion := "v0.0.0"
	if version != "" {
		if err := checkVersion(strings.TrimPrefix(version, "v"), version); err != nil {
			return err
		}
		requireVersion = version
	}
	manifest := "module example.com/release-consumer\n\ngo " + goVersion + "\n\nrequire " + modulePath + " " + requireVersion + "\n"
	if version == "" {
		manifest += "\nreplace " + modulePath + " => " + fmt.Sprintf("%q", filepath.ToSlash(root)) + "\n"
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte(manifest), 0600); err != nil {
		return err
	}
	source := `package main
import (
  "fmt"
  "strings"
  converter "github.com/Gk0Wk/openapi-thrift"
)
func main() {
  source := []byte(` + fmt.Sprintf("%q", smokeYAML) + `)
  validation, err := converter.Validate(source, converter.ValidationOptions{})
  if err != nil || validation.ErrorCount != 0 { panic(fmt.Sprintf("validation: %v %#v", err, validation)) }
  result, err := converter.Convert(source, converter.ProjectionOptions{Namespace: "release.smoke", ServiceName: "SmokeService"})
  if err != nil || !strings.Contains(result.Thrift, "service SmokeService") { panic(fmt.Sprintf("conversion: %v %s", err, result.Thrift)) }
  if _, err := converter.Convert([]byte("openapi: ["), converter.ProjectionOptions{}); err == nil { panic("invalid YAML accepted") }
  fmt.Println("Independent Go consumer passed")
}
`
	if err := os.WriteFile(filepath.Join(directory, "main.go"), []byte(source), 0600); err != nil {
		return err
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	run := func(args ...string) error {
		command := exec.Command(goBinary, args...)
		command.Dir = directory
		command.Env = replaceEnv(os.Environ(), "GOWORK", "off")
		command.Env = replaceEnv(command.Env, "GOBIN", directory)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		return command.Run()
	}
	if err := run("mod", "tidy"); err != nil {
		return err
	}
	if err := run("run", "."); err != nil {
		return err
	}
	install := modulePath + "/cmd/openapi-thrift"
	if version != "" {
		install += "@" + version
	}
	if err := run("install", install); err != nil {
		return err
	}
	name := "openapi-thrift"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	expected := "openapi-thrift "
	if version != "" {
		expected += version
	}
	return smokeBinary(filepath.Join(directory, name), directory, expected)
}
