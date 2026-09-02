package tracing

type Config struct {
	Enabled      bool    `mapstructure:"enabled"`
	Endpoint     string  `mapstructure:"endpoint"`
	SamplerRatio float64 `mapstructure:"sampler_ratio"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		Endpoint:     "localhost:4317",
		SamplerRatio: 1.0,
	}
}
