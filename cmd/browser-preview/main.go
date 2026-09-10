// browser-preview serves only the example and built browser assets on loopback.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	packageRoot := flag.String("package-root", ".", "Built source or installed npm package directory")
	flag.Parse()
	dist := filepath.Join(*packageRoot, "dist")
	if _, err := os.Stat(filepath.Join(dist, "openapi-thrift.wasm")); err != nil {
		log.Fatal("run pnpm build from the repository first: ", err)
	}
	assets := map[string]string{
		"/": "examples/browser/index.html", "/example.js": "examples/browser/example.js",
		"/dist/index.js": filepath.Join(dist, "index.js"), "/dist/wasm_exec.js": filepath.Join(dist, "wasm_exec.js"), "/dist/openapi-thrift.wasm": filepath.Join(dist, "openapi-thrift.wasm"),
	}
	mux := http.NewServeMux()
	for route, file := range assets {
		mux.HandleFunc("GET "+route, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != route {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src data:; connect-src 'self'")
			if filepath.Ext(file) == ".wasm" {
				w.Header().Set("Content-Type", "application/wasm")
			}
			http.ServeFile(w, r, file)
		})
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Browser example: http://%s\n", listener.Addr())
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
