// Package cli is the native command boundary; the reusable core has no file IO.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	core "github.com/Gk0Wk/openapi-thrift"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout, stderr); err != nil {
		if !errors.Is(err, errValidationFailed) {
			fmt.Fprintln(stderr, err)
		}
		return 1
	}
	return 0
}

var errValidationFailed = errors.New("OpenAPI validation failed")

func run(args []string, stdout, stderr io.Writer) error {
	action := "thrift"
	if len(args) > 0 && (args[0] == "validate" || args[0] == "thrift") {
		action, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("openapi-thrift", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var input, output, idlDir, namespace, service, profile string
	var strict, help bool
	for _, name := range []string{"input", "i"} {
		flags.StringVar(&input, name, "", "OpenAPI JSON or YAML input")
	}
	for _, name := range []string{"output", "o"} {
		flags.StringVar(&output, name, "", "Thrift output (default stdout)")
	}
	flags.StringVar(&idlDir, "idl-dir", "", "Existing IDL directory for route names")
	flags.StringVar(&namespace, "namespace", "", "Thrift Go namespace")
	flags.StringVar(&service, "service-name", "", "Thrift service name")
	flags.StringVar(&profile, "profile", core.ProfileApifoxHzThrift, "Validation profile")
	flags.BoolVar(&strict, "strict-warnings", false, "Fail validation on warnings")
	flags.BoolVar(&help, "help", false, "Show usage")
	flags.BoolVar(&help, "h", false, "Show usage")
	flags.Usage = func() { usage(stdout) }
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument: %s", flags.Arg(0))
	}
	if help {
		usage(stdout)
		return nil
	}
	if profile != core.ProfileApifoxHzThrift {
		return fmt.Errorf("unsupported profile: %s", profile)
	}
	if input == "" {
		usage(stdout)
		return errors.New("--input is required")
	}
	source, err := readLimited(input, 16<<20)
	if err != nil {
		return err
	}
	if action == "validate" {
		result, err := core.Validate(source, core.ValidationOptions{Profile: profile})
		if err != nil {
			return err
		}
		if len(result.Issues) > 0 {
			fmt.Fprintln(stdout, core.FormatValidationIssues(result.Issues))
		}
		status := "passed"
		if result.ErrorCount > 0 {
			status = "failed"
		} else if result.WarningCount > 0 {
			status = "passed with warnings"
		}
		fmt.Fprintf(stdout, "OpenAPI Render validation %s: %d paths, %d operations, %d schemas, %d errors, %d warnings\n", status, result.PathCount, result.OperationCount, result.SchemaCount, result.ErrorCount, result.WarningCount)
		if result.ErrorCount > 0 || (strict && result.WarningCount > 0) {
			return errValidationFailed
		}
		return nil
	}
	options := core.ProjectionOptions{Namespace: namespace, ServiceName: service}
	if idlDir != "" {
		options.RouteMethodNames, err = loadRouteNames(idlDir)
		if err != nil {
			return err
		}
	}
	result, err := core.Convert(source, options)
	if err != nil {
		return err
	}
	if output == "" {
		_, err = io.WriteString(stdout, result.Thrift)
		return err
	}
	if err := writeOutput(output, []byte(result.Thrift)); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, output)
	return err
}

func usage(out io.Writer) {
	fmt.Fprint(out, `openapi-thrift (Go core)

Usage:
  openapi-thrift validate --input <openapi.json|yaml> [--strict-warnings]
  openapi-thrift thrift --input <openapi.json|yaml> [--output <out.thrift>]
    [--idl-dir <directory>] [--namespace <go.namespace>] [--service-name <ServiceName>]

Local source checkout: go run ./cmd/openapi-thrift <command> <options>
`)
}
func readLimited(path string, limit int64) ([]byte, error) {
	if err := rejectSymlinks(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input is not a regular file: %s", path)
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("input exceeds %d bytes: %s", limit, path)
	}
	return body, nil
}
func loadRouteNames(root string) (map[string]string, error) {
	if err := rejectSymlinks(root); err != nil {
		return nil, err
	}
	files := []core.ThriftSourceFile{}
	remaining := int64(32 << 20)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("IDL symlinks are not allowed: %s", path)
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".thrift") {
			return nil
		}
		body, err := readLimited(path, remaining)
		if err != nil {
			return err
		}
		remaining -= int64(len(body))
		files = append(files, core.ThriftSourceFile{Path: path, Content: string(body)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return core.ExtractRouteMethodNames(files), nil
}
func rejectSymlinks(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for current := absolute; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("symlink path is not allowed: %s", current)
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	return nil
}
func writeOutput(path string, body []byte) (err error) {
	if err := rejectSymlinks(path); err != nil {
		return err
	}
	mode := fs.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output is not a regular file: %s", path)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".openapi-thrift-*")
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		removeErr := os.Remove(file.Name())
		if err == nil && closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			err = closeErr
		}
		if err == nil && removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = removeErr
		}
	}()
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
