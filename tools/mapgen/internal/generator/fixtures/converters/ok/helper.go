package ok

import (
	"time"

	m "github.com/agilsyofian/gosdk/utils/mapgen/runtime/mapper"
)

// registerMore exercises registration outside init through an aliased
// import; mapgen picks it up statically.
func registerMore() {
	m.Register(Truncate)
}

// Truncate converts a time into epoch seconds.
func Truncate(t time.Time) Timestamp {
	return Timestamp(t.Unix())
}
