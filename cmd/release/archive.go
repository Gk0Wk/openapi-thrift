package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type archiveFile struct {
	name string
	body []byte
	mode os.FileMode
}

func writeArchive(path string, files []archiveFile) (result error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	if strings.HasSuffix(path, ".zip") {
		writer := zip.NewWriter(file)
		defer func() { result = errors.Join(result, writer.Close()) }()
		for _, item := range files {
			header := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
			header.SetMode(item.mode)
			header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
			entry, err := writer.CreateHeader(header)
			if err != nil {
				return err
			}
			if _, err := entry.Write(item.body); err != nil {
				return err
			}
		}
		return nil
	}
	gz := gzip.NewWriter(file)
	defer func() { result = errors.Join(result, gz.Close()) }()
	writer := tar.NewWriter(gz)
	defer func() { result = errors.Join(result, writer.Close()) }()
	for _, item := range files {
		if err := writer.WriteHeader(&tar.Header{Name: item.name, Mode: int64(item.mode), Size: int64(len(item.body)), Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0)}); err != nil {
			return err
		}
		if _, err := writer.Write(item.body); err != nil {
			return err
		}
	}
	return nil
}

func readArchive(path string) (map[string]archiveFile, error) {
	files := map[string]archiveFile{}
	add := func(name string, mode os.FileMode, reader io.Reader) error {
		if name != "openapi-thrift" && name != "openapi-thrift.exe" && name != "release.json" && name != "LICENSE" {
			return fmt.Errorf("unexpected archive path %q", name)
		}
		if _, ok := files[name]; ok {
			return fmt.Errorf("duplicate archive path %q", name)
		}
		if !mode.IsRegular() {
			return fmt.Errorf("non-regular archive entry %q", name)
		}
		body, err := io.ReadAll(io.LimitReader(reader, 32<<20+1))
		if err != nil {
			return err
		}
		if len(body) > 32<<20 {
			return errors.New("archive member exceeds size limit")
		}
		files[name] = archiveFile{name, body, mode}
		return nil
	}
	if strings.HasSuffix(path, ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		for _, item := range reader.File {
			entry, err := item.Open()
			if err != nil {
				return nil, err
			}
			err = add(item.Name, item.Mode(), entry)
			closeErr := entry.Close()
			if err != nil || closeErr != nil {
				return nil, errors.Join(err, closeErr)
			}
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		gz, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader := tar.NewReader(gz)
		for {
			header, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			if header.Typeflag != tar.TypeReg {
				return nil, errors.New("non-regular tar entry")
			}
			if err := add(header.Name, os.FileMode(header.Mode), reader); err != nil {
				return nil, err
			}
		}
	}
	if len(files) != 3 {
		return nil, errors.New("archive must contain exactly a binary, LICENSE and release.json")
	}
	return files, nil
}

const smokeYAML = "openapi: 3.0.3\npaths:\n  /health:\n    get:\n      operationId: health\n      tags: [health]\n      responses:\n        204:\n          description: healthy\ncomponents:\n  schemas: {}\n"

func smokeArchive(path string) error {
	if path == "" {
		return errors.New("--archive is required")
	}
	files, err := readArchive(path)
	if err != nil {
		return err
	}
	var meta metadata
	if err := json.Unmarshal(files["release.json"].body, &meta); err != nil {
		return err
	}
	if err := checkVersion(meta.Version, ""); err != nil {
		return err
	}
	if meta.GOOS != runtime.GOOS || meta.GOARCH != runtime.GOARCH || meta.Dirty {
		return errors.New("archive target/clean-source identity does not match native smoke")
	}
	name := "openapi-thrift"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary, ok := files[name]
	if !ok || fmt.Sprintf("%x", sha256.Sum256(binary.body)) != meta.BinarySHA256 {
		return errors.New("archive binary checksum mismatch")
	}
	if runtime.GOOS != "windows" && binary.mode&0111 == 0 {
		return errors.New("archive lost executable permissions")
	}
	directory, err := os.MkdirTemp("", "openapi-thrift-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	binaryPath := filepath.Join(directory, name)
	if err := os.WriteFile(binaryPath, binary.body, binary.mode.Perm()); err != nil {
		return err
	}
	return smokeBinary(binaryPath, directory, "openapi-thrift v"+meta.Version+" ("+meta.Commit+")")
}

func smokeBinary(binaryPath, directory, expectedVersion string) error {
	invoke := func(args ...string) ([]byte, error) {
		command := exec.Command(binaryPath, args...)
		command.Dir = directory
		command.Env = replaceEnv(os.Environ(), "PATH", "")
		return command.CombinedOutput()
	}
	version, err := invoke("--version")
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(version)), expectedVersion) {
		return fmt.Errorf("installed CLI version: %q %v", version, err)
	}
	input, output := filepath.Join(directory, "input.yaml"), filepath.Join(directory, "output.thrift")
	if err := os.WriteFile(input, []byte(smokeYAML), 0600); err != nil {
		return err
	}
	if body, err := invoke("validate", "--input", input, "--strict-warnings"); err != nil {
		return fmt.Errorf("installed CLI validate: %w %s", err, body)
	}
	if body, err := invoke("thrift", "--input", input, "--namespace", "release.smoke", "--service-name", "SmokeService", "--output", output); err != nil {
		return fmt.Errorf("installed CLI convert: %w %s", err, body)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	if !bytes.Contains(before, []byte("namespace go release.smoke")) || !bytes.Contains(before, []byte("service SmokeService")) {
		return fmt.Errorf("unexpected installed CLI output: %s", before)
	}
	if err := os.WriteFile(input, []byte("openapi: ["), 0600); err != nil {
		return err
	}
	if _, err := invoke("thrift", "--input", input, "--output", output); err == nil {
		return errors.New("installed CLI accepted invalid YAML")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return errors.New("installed CLI destroyed output after invalid input")
	}
	fmt.Printf("Native installation passed: %s/%s; no Go/Node on PATH\n", runtime.GOOS, runtime.GOARCH)
	return nil
}
