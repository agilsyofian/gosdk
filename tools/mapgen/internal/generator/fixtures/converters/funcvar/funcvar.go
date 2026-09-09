// Package funcvar registers a variable of function type, which mapgen
// rejects.
package funcvar

import (
	"strconv"

	"github.com/agilsyofian/gosdk/tools/mapgen/runtime/mapper"
)

// Format is a converter held in a variable.
var Format = strconv.Itoa

func init() {
	mapper.Register(Format)
}
