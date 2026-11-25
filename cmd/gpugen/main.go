package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dstuden/gpugen/transpile"
	"golang.org/x/tools/go/packages"
)

func main() {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		log.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	failed := false
	for _, pkg := range pkgs {
		kernels, warnings := transpile.Find(pkg)
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, w)
		}

		for _, fd := range kernels {
			params, errs := transpile.Check(pkg, fd)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, e)
				}
				failed = true
				continue
			}

			name := fd.Name.Name
			src := transpile.Generate(name, params)
			if err := transpile.Validate(name, src); err != nil {
				fmt.Fprintf(os.Stderr, "%s: internal error: generated invalid WGSL for %s:\n%v\n",
					transpile.Where(pkg.Fset, fd.Pos()), name, err)
				failed = true
				continue
			}

			path := filepath.Join(filepath.Dir(pkg.GoFiles[0]), "shaders", name+".wgsl")
			if err := writeIfChanged(path, src); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s: %s -> shaders/%s.wgsl\n", transpile.Where(pkg.Fset, fd.Pos()), name, name)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func writeIfChanged(path, src string) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, []byte(src)) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(src), 0o644)
}
