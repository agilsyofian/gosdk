package generator_test

import (
	"fmt"
	"go/types"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/agilsyofian/gosdk/tools/mapgen/internal/generator"
)

const fixturePrefix = "github.com/agilsyofian/gosdk/tools/mapgen/internal/generator/fixtures/"

var loadFixtures = sync.OnceValues(func() (map[string]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, fixturePrefix+"...")
	if err != nil {
		return nil, fmt.Errorf("load fixture packages: %w", err)
	}
	byName := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("fixture %s has load errors: %v", pkg.PkgPath, pkg.Errors)
		}
		byName[strings.TrimPrefix(pkg.PkgPath, fixturePrefix)] = pkg
	}
	return byName, nil
})

func fixture(t *testing.T, name string) *packages.Package {
	t.Helper()
	pkgs, err := loadFixtures()
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	pkg, ok := pkgs[name]
	if !ok {
		t.Fatalf("fixture %q not loaded", name)
	}
	return pkg
}

func importedType(t *testing.T, pkg *packages.Package, path, name string) types.Type {
	t.Helper()
	for _, imp := range pkg.Types.Imports() {
		if imp.Path() == path {
			return imp.Scope().Lookup(name).Type()
		}
	}
	t.Fatalf("package %s does not import %s", pkg.PkgPath, path)
	return nil
}

func TestExtractConverters(t *testing.T) {
	t.Parallel()

	pkg := fixture(t, "converters/ok")
	table, err := generator.ExtractConverters([]*packages.Package{pkg}, "example.com/output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := table.Len(); got != 5 {
		t.Errorf("got %d converters, want 5", got)
	}

	scope := pkg.Types.Scope()
	userID := scope.Lookup("UserID").Type()
	timestamp := scope.Lookup("Timestamp").Type()
	timeT := importedType(t, pkg, "time", "Time")
	stringT := types.Typ[types.String]
	intT := types.Typ[types.Int]

	tests := []struct {
		name string
		src  types.Type
		dst  types.Type
		want generator.ConverterInfo
	}{
		{
			name: "ident argument with inferred type args",
			src:  userID,
			dst:  stringT,
			want: generator.ConverterInfo{Func: "FormatUserID", PkgPath: pkg.PkgPath},
		},
		{
			name: "explicit type args",
			src:  timestamp,
			dst:  timeT,
			want: generator.ConverterInfo{Func: "ToTime", PkgPath: pkg.PkgPath},
		},
		{
			name: "error-returning converter",
			src:  stringT,
			dst:  userID,
			want: generator.ConverterInfo{Func: "ParseUserID", PkgPath: pkg.PkgPath, HasErr: true},
		},
		{
			name: "function from another package",
			src:  stringT,
			dst:  intT,
			want: generator.ConverterInfo{Func: "Atoi", PkgPath: "strconv", HasErr: true},
		},
		{
			name: "registration outside init through aliased import",
			src:  timeT,
			dst:  timestamp,
			want: generator.ConverterInfo{Func: "Truncate", PkgPath: pkg.PkgPath},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := table.LookupInfo(tt.src, tt.dst)
			if !ok {
				t.Fatalf("converter from %s to %s not found", tt.src, tt.dst)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractConvertersLookupMiss(t *testing.T) {
	t.Parallel()

	pkg := fixture(t, "converters/ok")
	table, err := generator.ExtractConverters([]*packages.Package{pkg}, "example.com/output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := table.LookupInfo(types.Typ[types.Int], types.Typ[types.String]); ok {
		t.Error("expected lookup miss for unregistered pair")
	}
}

func TestExtractConvertersErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		wantErr string
	}{
		{"converters/dup", "already registered"},
		{"converters/closure", "function literal"},
		{"converters/unexported", "must be exported"},
		{"converters/genericfn", "generic converter functions are not supported"},
		{"converters/funcvar", "must be a named function"},
		{"converters/method", "must not be a method"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()
			pkg := fixture(t, tt.fixture)
			_, err := generator.ExtractConverters([]*packages.Package{pkg}, "example.com/output")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestExtractConvertersUnexportedInOutputPackage(t *testing.T) {
	t.Parallel()

	pkg := fixture(t, "converters/unexported")
	table, err := generator.ExtractConverters([]*packages.Package{pkg}, pkg.PkgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := table.Len(); got != 1 {
		t.Errorf("got %d converters, want 1", got)
	}
}
