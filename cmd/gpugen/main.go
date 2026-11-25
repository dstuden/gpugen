package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

const kernelDirective = "//gpu:kernel"

type paramKind int

const (
	paramID paramKind = iota
	paramScalar
	paramSlice
)

func (k paramKind) String() string {
	return [...]string{"id", "scalar", "slice"}[k]
}

type param struct {
	name string
	kind paramKind
	typ  types.BasicKind // scalar type, or element type of a slice
}

func findKernels(pkg *packages.Package) []*ast.FuncDecl {
	var kernels []*ast.FuncDecl
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Doc == nil {
				continue
			}

			for _, comment := range fd.Doc.List {
				text := strings.TrimSpace(comment.Text)
				if text == kernelDirective {
					kernels = append(kernels, fd)
					break
				}
				if strings.Contains(text, "gpu:kernel") {
					fmt.Fprintf(os.Stderr, "%s: warning: did you mean %s?\n",
						where(pkg.Fset, comment.Pos()), kernelDirective)
				}
			}
		}
	}
	return kernels
}

func checkKernel(pkg *packages.Package, fd *ast.FuncDecl) ([]param, []string) {
	var errs []string
	errorf := func(p token.Pos, format string, args ...any) {
		errs = append(errs, where(pkg.Fset, p)+": "+fmt.Sprintf(format, args...))
	}

	name := fd.Name.Name
	fn, ok := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
	if !ok {
		errorf(fd.Pos(), "no type information for %s", name)
		return nil, errs
	}
	sig := fn.Signature()

	if sig.Recv() != nil {
		errorf(fd.Pos(), "%s is a method; kernels must be plain functions", name)
	}
	if sig.TypeParams().Len() > 0 {
		errorf(fd.Pos(), "%s is generic; kernels can't have type parameters", name)
	}
	if sig.Results().Len() > 0 {
		errorf(fd.Pos(), "%s returns values; write results into a slice parameter instead", name)
	}
	if sig.Variadic() {
		errorf(fd.Pos(), "%s is variadic; kernels can't be", name)
	}

	params := sig.Params()
	if params.Len() == 0 {
		errorf(fd.Pos(), "%s has no parameters; the first must be the thread index (id uint32)", name)
		return nil, errs
	}

	var out []param
	for i := range params.Len() {
		v := params.At(i)

		if i == 0 {
			if b, ok := v.Type().Underlying().(*types.Basic); !ok || b.Kind() != types.Uint32 {
				errorf(v.Pos(), "first parameter %s must be the thread index of type uint32, not %s", v.Name(), v.Type())
			}
			out = append(out, param{v.Name(), paramID, types.Uint32})
			continue
		}

		if s, ok := v.Type().Underlying().(*types.Slice); ok {
			kind, why := scalarKind(s.Elem())
			if why != "" {
				errorf(v.Pos(), "parameter %s: element type %s", v.Name(), why)
			}
			out = append(out, param{v.Name(), paramSlice, kind})
			continue
		}

		kind, why := scalarKind(v.Type())
		if why != "" {
			errorf(v.Pos(), "parameter %s: %s", v.Name(), why)
		}
		out = append(out, param{v.Name(), paramScalar, kind})
	}
	return out, errs
}

// scalarKind returns the basic kind of t if kernels accept it, or a reason why not.
func scalarKind(t types.Type) (types.BasicKind, string) {
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return 0, fmt.Sprintf("%s isn't supported; use float32, uint32, int32 or a slice of them", t)
	}
	switch b.Kind() {
	case types.Float32, types.Uint32, types.Int32:
		return b.Kind(), ""
	case types.Int, types.Uint, types.Int64, types.Uint64, types.Uintptr:
		return 0, fmt.Sprintf("%s is 64-bit on the host; use int32 or uint32", t)
	case types.Float64:
		return 0, fmt.Sprintf("%s is 64-bit; use float32", t)
	case types.Int8, types.Int16, types.Uint8, types.Uint16:
		return 0, fmt.Sprintf("%s is narrower than 32 bits; use int32 or uint32", t)
	case types.Bool:
		return 0, "bool isn't supported; use uint32 (0 or 1)"
	}
	return 0, fmt.Sprintf("%s isn't supported; use float32, uint32 or int32", t)
}

func where(fset *token.FileSet, p token.Pos) string {
	pos := fset.Position(p)
	return fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line)
}

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
		for _, fd := range findKernels(pkg) {
			params, errs := checkKernel(pkg, fd)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, e)
				}
				failed = true
				continue
			}

			fmt.Printf("%s: kernel %s\n", where(pkg.Fset, fd.Pos()), fd.Name.Name)
			for _, p := range params {
				typ := types.Typ[p.typ].Name()
				if p.kind == paramSlice {
					typ = "[]" + typ
				}
				fmt.Printf("\t%-8s %-6s %s\n", p.name, p.kind, typ)
			}
		}
	}
	if failed {
		os.Exit(1)
	}
}
