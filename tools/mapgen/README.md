# mapgen

> [!IMPORTANT]
> This repository is archived. Development continues in [kanna](https://github.com/go-kanna/kanna), where mapgen became [`kanna-mapper`](https://github.com/go-kanna/kanna#kanna-mapper).

Type-safe struct mapping code generator for Go, driven by registered converters and `go generate`.

mapgen generates the boring mapping functions between your domain models and wire types (protobuf messages, DTOs) that you would otherwise write by hand. There is no reflection at runtime: the generated code calls your converter functions directly and compiles like hand-written code.

```go
// What you write: a converter package registered once.
func init() {
	mapper.Register(ToDate)      // time.Time -> *date.Date
	mapper.Register(UUIDToString)
	mapper.Register(ToTime)      // *date.Date -> time.Time
	mapper.RegisterE(uuid.Parse) // string -> uuid.UUID, can fail
}
```

```go
// What you add: one directive in the package that should hold the mappers.
//go:generate go tool mapgen -types=model.Employee:*employeev1.Employee -converter-pkg=./lib/converters
```

```go
// What you get: readable, compile-checked mapping functions.
func EmployeeToEmployeev1(src model.Employee) *employeev1.Employee {
	return &employeev1.Employee{
		Id:      converters.UUIDToString(src.ID),
		Name:    src.Name,
		HiredAt: converters.ToDate(src.HiredAt),
	}
}

func EmployeeFromEmployeev1(src *employeev1.Employee) (model.Employee, error) {
	if src == nil {
		return model.Employee{}, nil
	}
	v1, err0 := uuid.Parse(src.GetId())
	if err0 != nil {
		return model.Employee{}, fmt.Errorf("map model.Employee.ID: %w", err0)
	}
	// ...
}
```

See [examples](./examples) for a complete, runnable module.

## Installation

Requires Go 1.25+.

```sh
go get -tool github.com/agilsyofian/gosdk/tools/mapgen@latest
```

## How it works

`mapper.Register` calls are never executed by the generator. Instead, mapgen statically analyzes the converter package, extracts the registered `(Src, Dst)` type pairs and function references, and wires those functions directly into the generated code. The `runtime/mapper` package is a declaration DSL first; the registry also works at runtime through `mapper.Convert` if you need dynamic lookup.

Package selectors in `-types` (`model`, `employeev1`) are resolved from the imports of the package containing the directive, with the directive file's imports taking priority. If the package already imports the types for real use, the directive is a single line. Otherwise keep a minimal directive file:

```go
//go:generate go tool mapgen -types=model.Employee:*employeev1.Employee -converter-pkg=./lib/converters
package handler

import (
	_ "github.com/acme/app/gen/employee/v1"
	_ "github.com/acme/app/internal/model"
)
```

Full import paths are also accepted: `-types=github.com/acme/app/internal/model.Employee:*github.com/acme/app/gen/employee/v1.Employee`.

## Flags

| Flag                        | Description                                                                                         |
|-----------------------------|-----------------------------------------------------------------------------------------------------|
| `-types=SRC:DST[,...]`      | Type pairs to map. Optional `*` prefix for pointer types. Repeatable.                               |
| `-converter-pkg=PATH`       | Package containing `mapper.Register` calls (directory or import path). Repeatable.                  |
| `-output=PATH`              | Output directory (file name derives from `$GOFILE`), or a `.go` file path. Default `.`.             |
| `-direction=both\|to\|from` | Which functions to generate. Default `both`.                                                        |
| `-ignore=TYPE.FIELD[,...]`  | Skip destination fields on types you cannot tag (e.g., `employeev1.Employee.Internal`). Repeatable. |
| `-package=NAME`             | Output package name when `$GOPACKAGE` is not available.                                             |
| `-check`                    | Verify generated files are up to date instead of writing (for CI).                                  |

One pair `A:B` generates both `AToB` and `AFromB` style functions, named from the A side (`EmployeeToEmployeev1` / `EmployeeFromEmployeev1`). A function returns `(T, error)` only when it uses a fallible converter registered with `RegisterE`.

## Field matching

For each destination field, mapgen picks the source in this order:

1. `map` tag on either side (see below)
2. exact name match
3. case-insensitive match (`ID` ↔ `Id`)
4. promoted (embedded) fields, exact match only

Reading prefers nil-safe getters (`GetName()`) when present, which handles protobuf pointers and proto3 optional fields naturally. Unexported fields are always skipped, so protobuf bookkeeping fields (`state`, `sizeCache`, `unknownFields`) never get in the way.

Every remaining destination field must resolve, or generation fails with the field's position and a copy-pasteable suggestion:

```
internal/model/employee.go:12:2: cannot map model.Employee.ID (uuid.UUID) to employeev1.Employee.Id (string)
	register a converter: mapper.Register(func(uuid.UUID) string { ... })
	or declare the pair in -types, or exclude the field with map:"-" or -ignore
```

### The `map` struct tag

```go
type Employee struct {
	ID        uuid.UUID `map:"Id"` // maps to the counterpart field named Id
	CreatedAt time.Time `map:"-"`  // invisible to mapgen
}
```

`map:"Name"` names the counterpart field and works in both directions. `map:"-"` removes the field from mapping entirely. Tags naming nonexistent counterparts are errors, so typos surface at generation time. For types you cannot tag (generated protobuf code), use `-ignore`.

## Conversion rules

For a source value of type S and a destination field of type D, mapgen resolves in order:

1. identical types: direct assignment
2. registered converter `(S, D)`: direct call
3. declared pair in `-types`: call to the generated mapper (errors propagate)
4. `*S`: dereference, leaving the destination zero when nil (nil pointers map to nil pointers, never to a pointer to a zero value)
5. `*D`: take the address of the converted value
6. slices: element-wise conversion; nil stays nil
7. safe Go conversions only: loss-free numeric widening (`int32` → `int64`, `uint8` → `int16`, `float32` → `float64`, exactly representable integers → float), named ↔ underlying (e.g., proto enum ↔ int32), string ↔ []byte

Lossy or surprising conversions are deliberately rejected: numeric ↔ string (`string(rune(42))` is never what you want), float → integer, and narrowing or sign-changing integer conversions (`int64` → `int32`, `int` → `uint`) all require an explicit converter.

## Limitations (v1)

- map-typed fields, protobuf `oneof`, the protobuf opaque API, and generic types are not supported; errors name them explicitly and `-ignore` is the escape hatch
- converters must be named functions callable from the generated code (no closures, no methods)
- proto3 optional: an unset field and a zero value collapse to the same thing on a roundtrip through a getter
- pointer fields assigned directly (identical types) share the pointee; converters and declared pairs produce copies

## License

[MIT](./LICENSE)
