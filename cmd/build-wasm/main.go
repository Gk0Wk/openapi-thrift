// build-wasm creates browser artifacts from the same Go module as the CLI.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	module, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("run from the openapi-thrift repository: %w", err)
	}
	if !strings.Contains(string(module), "\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n") {
		return fmt.Errorf("WASM build must use the Go version pinned in go.mod; got %s", runtime.Version())
	}
	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}
	// Remove only compiler-owned outputs of retired sources. Never recursively
	// delete dist or an arbitrary output directory while preparing a build.
	for _, base := range []string{"cli", "profile", "projector", "thrift-route-index"} {
		for _, extension := range []string{".js", ".d.ts"} {
			if err := os.Remove(filepath.Join("dist", base+extension)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", "dist/openapi-thrift.wasm", "./cmd/openapi-thrift-wasm")
	command.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0", "GOWORK=off")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	for source, target := range map[string]string{
		filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"): "dist/wasm_exec.js",
		"src/wasm_exec.d.ts": "dist/wasm_exec.d.ts",
	} {
		body, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0644); err != nil {
			return err
		}
	}
	fmt.Println("Built dist/openapi-thrift.wasm with " + runtime.Version())
	return nil
}
