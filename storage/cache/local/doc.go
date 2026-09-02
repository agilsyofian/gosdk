// Package local provides an in-memory implementation of the cache.Cache interface.
//
// It supports TTL-based expiration, atomic operations (SetNX, Incr), prefix deletion,
// JSON marshaling/unmarshaling, background cleanup, OpenTelemetry tracing, and SDK logging.
package local
