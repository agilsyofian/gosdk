package redis

import "time"

// Mode indicates which Redis topology to use.
type Mode string

const (
	// ModeStandalone connects to a single Redis node.
	ModeStandalone Mode = "standalone"
	// ModeCluster connects to a Redis Cluster (multiple shard nodes).
	ModeCluster Mode = "cluster"
	// ModeSentinel connects to Redis via Sentinel for high-availability failover.
	ModeSentinel Mode = "sentinel"
)

// Config defines the configuration for Redis connection.
type Config struct {
	// Type selects the connection topology: standalone, cluster, or sentinel.
	// Defaults to "standalone".
	Type Mode `mapstructure:"type" validate:"required,oneof=standalone cluster sentinel"`

	// Addrs is the list of Redis addresses.
	//   - Standalone: ["host:port"]
	//   - Cluster:    ["node1:port", "node2:port", ...]
	//   - Sentinel:   ["sentinel1:port", "sentinel2:port", ...]
	Addrs []string `mapstructure:"addrs" validate:"required,min=1"`

	// MasterName is the Sentinel master name (required for sentinel mode).
	MasterName string `mapstructure:"master_name"`

	// Username and Password for Redis AUTH.
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`

	// DB selects the logical database (standalone only; ignored in cluster/sentinel).
	DB int `mapstructure:"db"`

	// PoolSize is the maximum number of socket connections per node. Default: 10.
	PoolSize int `mapstructure:"pool_size"`
	// MinIdleConns is the minimum number of idle connections maintained. Default: 2.
	MinIdleConns int `mapstructure:"min_idle_conns"`

	// DialTimeout is the timeout for establishing new connections. Default: 5s.
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
	// ReadTimeout is the timeout for reading replies. Default: 3s.
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
	// WriteTimeout is the timeout for writing commands. Default: 3s.
	WriteTimeout time.Duration `mapstructure:"write_timeout"`

	// MaxRetries is the maximum number of retries before giving up. Default: 3.
	MaxRetries int `mapstructure:"max_retries"`
	// MinRetryBackoff is the minimum backoff between retries. Default: 8ms.
	MinRetryBackoff time.Duration `mapstructure:"min_retry_backoff"`
	// MaxRetryBackoff is the maximum backoff between retries. Default: 512ms.
	MaxRetryBackoff time.Duration `mapstructure:"max_retry_backoff"`

	// Cluster-specific options.
	// MaxRedirects specifies how many times to follow MOVED/ASK redirections. Default: 8.
	MaxRedirects int `mapstructure:"max_redirects"`
	// RouteByLatency enables routing to the lowest latency node (cluster only).
	RouteByLatency bool `mapstructure:"route_by_latency"`
	// RouteRandomly enables random routing to a node (cluster only).
	RouteRandomly bool `mapstructure:"route_randomly"`
	// ReadOnly enables read-only commands on replica nodes (cluster only).
	ReadOnly bool `mapstructure:"read_only"`

	Tracing struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"tracing"`

	Logger struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"logger"`
}

// DefaultConfig returns a sensible default config for standalone mode.
func DefaultConfig() Config {
	return Config{
		Type:            ModeStandalone,
		Addrs:           []string{"localhost:6379"},
		PoolSize:        10,
		MinIdleConns:    2,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		MaxRedirects:    8,
	}
}
