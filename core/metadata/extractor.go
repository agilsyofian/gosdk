package metadata

import "strings"

// Carrier represents an abstract key-value carrier from transport protocols
// (such as http.Header, grpc/metadata.MD, or plain maps).
type Carrier interface {
	Get(key string) string
	Keys() []string
}

// MapCarrier adapts a plain map[string]string to the Carrier interface.
type MapCarrier map[string]string

// Get returns the value for key (case-insensitive).
func (m MapCarrier) Get(key string) string {
	if m == nil {
		return ""
	}
	val, ok := m[key]
	if ok {
		return val
	}
	lower := strings.ToLower(key)
	for k, v := range m {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}

// Keys returns all keys in the map.
func (m MapCarrier) Keys() []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// FuncCarrier adapts a function and key list into a Carrier.
type FuncCarrier struct {
	GetFunc  func(string) string
	KeysFunc func() []string
}

func (f FuncCarrier) Get(key string) string {
	if f.GetFunc != nil {
		return f.GetFunc(key)
	}
	return ""
}

func (f FuncCarrier) Keys() []string {
	if f.KeysFunc != nil {
		return f.KeysFunc()
	}
	return nil
}

// Extract extracts standard metadata and dynamic baggage from a Carrier using Config.
func Extract(c Carrier, cfgs ...Config) *Metadata {
	cfg := DefaultConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0].Normalize()
	}

	meta := New()
	if c == nil {
		return meta
	}

	// 1. Extract standard configured headers
	meta.Authorization = c.Get(cfg.AuthHeader)
	meta.TraceID = c.Get(cfg.TraceIDHeader)
	if strings.EqualFold(cfg.TraceIDHeader, "traceparent") || strings.HasPrefix(meta.TraceID, "00-") {
		meta.TraceID = parseTraceparent(meta.TraceID)
	}
	meta.SpanID = c.Get(cfg.SpanIDHeader)
	meta.RequestID = c.Get(cfg.RequestIDHeader)
	meta.SessionID = c.Get(cfg.SessionIDHeader)
	meta.UserID = c.Get(cfg.UserIDHeader)
	meta.ClientIP = c.Get(cfg.ClientIPHeader)
	meta.Timestamp = c.Get(cfg.TimestampHeader)

	// Trace ID fallback (W3C traceparent or correlation ID)
	if meta.TraceID == "" {
		if tp := c.Get("traceparent"); tp != "" {
			meta.TraceID = parseTraceparent(tp)
		} else if corr := c.Get("X-Correlation-Id"); corr != "" {
			meta.TraceID = corr
		}
	}

	// 2. Extract explicitly configured capture headers
	for _, header := range cfg.CaptureHeaders {
		val := c.Get(header)
		if val != "" {
			meta.Set(header, val)
		}
	}

	// 3. Extract prefix-matched headers
	allKeys := c.Keys()
	for _, key := range allKeys {
		lowerKey := strings.ToLower(key)
		for _, prefix := range cfg.CapturePrefixes {
			if strings.HasPrefix(lowerKey, strings.ToLower(prefix)) {
				val := c.Get(key)
				if val != "" {
					meta.Set(lowerKey, val)
				}
				break
			}
		}
	}

	return meta
}

// parseTraceparent extracts the 32-char trace ID from W3C traceparent (version-traceid-parentid-traceflags).
func parseTraceparent(tp string) string {
	parts := strings.Split(strings.TrimSpace(tp), "-")
	if len(parts) >= 2 && len(parts[1]) == 32 {
		return parts[1]
	}
	return tp
}
