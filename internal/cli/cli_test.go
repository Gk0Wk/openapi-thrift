package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeCLIWithoutNodePath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	fixture := filepath.Join("..", "..", "tests", "fixtures", "apifox-boundary-lab.supported.openapi.json")
	valid := filepath.Join(t.TempDir(), "valid.yaml")
	if err := os.WriteFile(valid, []byte("openapi: 3.0.3\npaths:\n  /health:\n    get:\n      operationId: health\n      tags: [health]\n      responses:\n        204:\n          description: healthy\ncomponents:\n  schemas: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := Run([]string{"validate", "--input", valid}, &out, &diagnostic); code != 0 {
		t.Fatalf("%d: %s %s", code, &out, &diagnostic)
	}
	output := filepath.Join(t.TempDir(), "api.thrift")
	if code := Run([]string{"thrift", "--input", fixture, "--output", output}, &out, &diagnostic); code != 0 {
		t.Fatalf("%d: %s", code, &diagnostic)
	}
	body, err := os.ReadFile(output)
	if err != nil || !bytes.Contains(body, []byte("service ")) {
		t.Fatalf("missing output: %v", err)
	}
	// Replacement is a normal CLI workflow on Windows as well as POSIX.
	if code := Run([]string{"thrift", "--input", fixture, "--output", output}, &out, &diagnostic); code != 0 {
		t.Fatalf("replacement failed: %s", &diagnostic)
	}
}
func TestCLIInvalidContractPreservesExistingOutput(t *testing.T) {
	directory := t.TempDir()
	input, output := filepath.Join(directory, "input.yaml"), filepath.Join(directory, "api.thrift")
	if err := os.WriteFile(input, []byte("openapi: 3.0.3\npaths:\n  /empty:\n    get:\n      responses: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("preserve me"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := Run([]string{"thrift", "-i", input, "-o", output}, &out, &diagnostic); code == 0 {
		t.Fatal("invalid input accepted")
	}
	body, err := os.ReadFile(output)
	if err != nil || string(body) != "preserve me" {
		t.Fatalf("existing output changed: %q %v", body, err)
	}
}
func TestCLIFlagsAndStrictWarnings(t *testing.T) {
	for _, args := range [][]string{{}, {"validate", "--input"}, {"--profile", "unknown"}, {"--unknown"}, {"thrift", "trailing"}} {
		var out, diagnostic bytes.Buffer
		if Run(args, &out, &diagnostic) == 0 {
			t.Errorf("invalid args accepted: %v", args)
		}
	}
	input := filepath.Join(t.TempDir(), "input.yaml")
	if err := os.WriteFile(input, []byte("openapi: 3.0.3\npaths:\n  /health:\n    get:\n      operationId: health\n      responses:\n        204:\n          description: healthy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if Run([]string{"validate", "--input", input}, &out, &diagnostic) != 0 {
		t.Fatalf("warnings failed: %s %s", &out, &diagnostic)
	}
	if Run([]string{"validate", "--input", input, "--strict-warnings"}, &out, &diagnostic) == 0 {
		t.Fatal("strict warnings accepted")
	}
	if !strings.Contains(out.String(), "passed with warnings") {
		t.Fatal("missing warning summary")
	}
}

func TestCLIRefusesDirectoryJunctionInputsAndOutputs(t *testing.T) {
	root := t.TempDir()
	outside, link := filepath.Join(root, "outside"), filepath.Join(root, "linked")
	if err := os.Mkdir(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "api.thrift"), []byte("preserve me"), 0600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if body, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
			t.Fatalf("junction: %s %v", body, err)
		}
	} else if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := writeOutput(filepath.Join(link, "api.thrift"), []byte("overwrite")); err == nil {
		t.Fatal("junction output accepted")
	}
	if _, err := readLimited(filepath.Join(link, "api.thrift"), 1024); err == nil {
		t.Fatal("junction input accepted")
	}
	if _, err := loadRouteNames(link); err == nil {
		t.Fatal("junction IDL root accepted")
	}
	if body, err := os.ReadFile(filepath.Join(outside, "api.thrift")); err != nil || string(body) != "preserve me" {
		t.Fatalf("outside changed: %s %v", body, err)
	}
}
