package cli

import (
	"errors"
	"flag"
	"fmt"
	"go/token"
	"io"
	"strings"
)

// Parse parses command-line arguments into a Config. Usage and error
// messages produced by flag parsing are written to errOutput.
func Parse(args []string, errOutput io.Writer) (Config, error) {
	fs := flag.NewFlagSet("mapgen", flag.ContinueOnError)
	fs.SetOutput(errOutput)

	var types, converterPkgs, ignores listFlag
	fs.Var(&types, "types", "comma-separated SRC:DST type pairs (e.g., model.Employee:*employeev1.Employee); repeatable")
	fs.Var(&converterPkgs, "converter-pkg", "package containing mapper.Register calls; repeatable")
	fs.Var(&ignores, "ignore", "comma-separated destination fields to skip (e.g., model.Employee.CreatedAt); repeatable")
	output := fs.String("output", ".", "output directory, or a file path ending in .go")
	direction := fs.String("direction", string(DirectionBoth), `which functions to generate: "both", "to", or "from"`)
	pkgName := fs.String("package", "", "output package name (defaults to $GOPACKAGE)")
	check := fs.Bool("check", false, "verify generated files are up to date instead of writing them")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse arguments: %w", err)
	}

	if len(types.values) == 0 {
		return Config{}, errors.New("-types is required")
	}

	cfg := Config{
		ConverterPkgs: converterPkgs.values,
		Output:        *output,
		Package:       *pkgName,
		Check:         *check,
	}
	for _, s := range types.values {
		pair, err := parsePair(s)
		if err != nil {
			return Config{}, err
		}
		cfg.Pairs = append(cfg.Pairs, pair)
	}
	for _, s := range ignores.values {
		ref, err := parseFieldRef(s)
		if err != nil {
			return Config{}, err
		}
		cfg.Ignores = append(cfg.Ignores, ref)
	}
	d, err := parseDirection(*direction)
	if err != nil {
		return Config{}, err
	}
	cfg.Direction = d
	if cfg.Package != "" && !token.IsIdentifier(cfg.Package) {
		return Config{}, fmt.Errorf("invalid -package %q", cfg.Package)
	}
	if cfg.Output == "" {
		return Config{}, errors.New("-output must not be empty")
	}
	return cfg, nil
}

func parsePair(s string) (TypePair, error) {
	src, dst, ok := strings.Cut(s, ":")
	if !ok || src == "" || dst == "" || strings.Contains(dst, ":") {
		return TypePair{}, fmt.Errorf("invalid -types entry %q: want SRC:DST", s)
	}
	srcRef, err := parseTypeRef(src)
	if err != nil {
		return TypePair{}, fmt.Errorf("invalid -types entry %q: %w", s, err)
	}
	dstRef, err := parseTypeRef(dst)
	if err != nil {
		return TypePair{}, fmt.Errorf("invalid -types entry %q: %w", s, err)
	}
	return TypePair{Src: srcRef, Dst: dstRef}, nil
}

func parseTypeRef(s string) (TypeRef, error) {
	var ref TypeRef
	rest := strings.TrimPrefix(s, "*")
	ref.Pointer = rest != s
	if i := strings.LastIndex(rest, "."); i >= 0 {
		ref.Pkg = rest[:i]
		ref.Name = rest[i+1:]
	}
	if ref.Name == "" {
		ref.Name = rest
	}
	if !token.IsIdentifier(ref.Name) {
		return TypeRef{}, fmt.Errorf("%q is not a valid type name", ref.Name)
	}
	if ref.Pkg == "" && strings.Contains(rest, ".") {
		return TypeRef{}, fmt.Errorf("%q has an empty package selector", s)
	}
	if ref.Pkg != "" && !ref.IsImportPath() && !token.IsIdentifier(ref.Pkg) {
		return TypeRef{}, fmt.Errorf("%q is not a valid package selector", ref.Pkg)
	}
	return ref, nil
}

func parseFieldRef(s string) (FieldRef, error) {
	i := strings.LastIndex(s, ".")
	if i < 0 {
		return FieldRef{}, fmt.Errorf("invalid -ignore entry %q: want TYPE.FIELD", s)
	}
	typeSpec, field := s[:i], s[i+1:]
	if !token.IsIdentifier(field) {
		return FieldRef{}, fmt.Errorf("invalid -ignore entry %q: %q is not a valid field name", s, field)
	}
	ref, err := parseTypeRef(typeSpec)
	if err != nil {
		return FieldRef{}, fmt.Errorf("invalid -ignore entry %q: %w", s, err)
	}
	if ref.Pointer {
		return FieldRef{}, fmt.Errorf("invalid -ignore entry %q: pointer marker is not allowed", s)
	}
	return FieldRef{Type: ref, Field: field}, nil
}

func parseDirection(s string) (Direction, error) {
	switch d := Direction(s); d {
	case DirectionBoth, DirectionTo, DirectionFrom:
		return d, nil
	default:
		return "", fmt.Errorf(`invalid -direction %q: want "both", "to", or "from"`, s)
	}
}

type listFlag struct {
	values []string
}

var _ flag.Value = (*listFlag)(nil)

func (f *listFlag) String() string {
	return strings.Join(f.values, ",")
}

func (f *listFlag) Set(s string) error {
	for v := range strings.SplitSeq(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			f.values = append(f.values, v)
		}
	}
	return nil
}
