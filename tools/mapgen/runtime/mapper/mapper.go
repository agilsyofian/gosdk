// Package mapper declares type converters for the mapgen code generator.
//
// Converters registered in a package's init function are discovered by
// static analysis when mapgen runs; generated code calls them directly
// without going through the registry. The registry is also usable at
// runtime through Convert for manual conversions.
package mapper

import (
	"fmt"
	"reflect"
	"sync"
)

type key struct {
	src, dst reflect.Type
}

var (
	mu       sync.RWMutex
	registry = make(map[key]any)
)

// Register registers fn as the converter from Src to Dst.
// It panics if fn is nil or a converter for the same type pair is already
// registered.
func Register[Src, Dst any](fn func(Src) Dst) {
	if fn == nil {
		panic("mapper: nil converter")
	}
	RegisterE(func(src Src) (Dst, error) {
		return fn(src), nil
	})
}

// RegisterE registers fn as the converter from Src to Dst for conversions
// that can fail.
// It panics if fn is nil or a converter for the same type pair is already
// registered.
func RegisterE[Src, Dst any](fn func(Src) (Dst, error)) {
	if fn == nil {
		panic("mapper: nil converter")
	}
	k := key{src: reflect.TypeFor[Src](), dst: reflect.TypeFor[Dst]()}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[k]; ok {
		panic(fmt.Sprintf("mapper: converter from %s to %s is already registered", k.src, k.dst))
	}
	registry[k] = fn
}

// Convert converts src using the registered converter from Src to Dst.
func Convert[Src, Dst any](src Src) (Dst, error) {
	k := key{src: reflect.TypeFor[Src](), dst: reflect.TypeFor[Dst]()}
	mu.RLock()
	v, ok := registry[k]
	mu.RUnlock()
	if !ok {
		var zero Dst
		return zero, fmt.Errorf("mapper: no converter registered from %s to %s", k.src, k.dst)
	}
	fn, ok := v.(func(Src) (Dst, error))
	if !ok {
		var zero Dst
		return zero, fmt.Errorf("mapper: converter from %s to %s has unexpected type %T", k.src, k.dst, v)
	}
	return fn(src)
}
