package transpile

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGolden(t *testing.T) {
	const dir = "testdata/kernels"
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, "./"+dir)
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal(dir + " doesn't compile")
	}
	pkg := pkgs[0]

	kernels, warnings := Find(pkg)
	for _, w := range warnings {
		t.Error(w)
	}
	if len(kernels) == 0 {
		t.Fatal("no kernels found in " + dir)
	}

	for _, fd := range kernels {
		name := fd.Name.Name
		t.Run(name, func(t *testing.T) {
			params, errs := Check(pkg, fd)
			if len(errs) > 0 {
				t.Fatal(strings.Join(errs, "\n"))
			}
			got := Generate(name, params)
			if err := Validate(name, got); err != nil {
				t.Fatalf("invalid WGSL:\n%v\n%s", err, got)
			}

			path := filepath.Join(dir, name+".wgsl")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run: go test ./transpile -update)", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s (run with -update if intended)\n--- got:\n%s--- want:\n%s", path, got, want)
			}
		})
	}
}
