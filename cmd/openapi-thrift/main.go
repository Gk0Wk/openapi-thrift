package main

import (
	"github.com/Gk0Wk/openapi-thrift/internal/cli"
	"os"
)

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }
