// Package genericfn registers generic converters, which mapgen rejects.
package genericfn

import (
	"github.com/agilsyofian/gosdk/tools/mapgen/runtime/mapper"
)

func init() {
	mapper.Register(identity[int])
	mapper.Register[string, string](identity)
}

func identity[T any](v T) T {
	return v
}
