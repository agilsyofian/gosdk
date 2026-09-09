// Command mapgen generates type-to-type mapping functions driven by
// registered converters. It is meant to run under go generate:
//
//	//go:generate go tool mapgen -types=model.Employee:*employeev1.Employee -converter-pkg=./lib/converters
package main

import (
	"fmt"
	"os"

	"github.com/agilsyofian/gosdk/tools/mapgen/internal/cli"
	"github.com/agilsyofian/gosdk/tools/mapgen/internal/generator"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:], os.Stderr)
	if err != nil {
		fail(err)
	}
	dir, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	env := generator.Env{
		GoFile:    os.Getenv("GOFILE"),
		GoPackage: os.Getenv("GOPACKAGE"),
		Dir:       dir,
	}
	if err := generator.Run(cfg, env); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mapgen:", err)
	os.Exit(1)
}
