package gcs

// Config holds configuration settings for Google Cloud Storage.
type Config struct {
	// Bucket is the name of the target GCS bucket (required).
	Bucket string `mapstructure:"bucket" yaml:"bucket" json:"bucket"`

	// CredentialsJSON is the raw JSON string or base64-encoded string of the service account key (optional).
	// If empty and CredentialsFile is also empty, Application Default Credentials (ADC) or
	// Workload Identity will be used automatically.
	CredentialsJSON string `mapstructure:"credentials_json" yaml:"credentials_json" json:"credentials_json"`

	// CredentialsFile is the file path to the service account JSON key file (optional).
	CredentialsFile string `mapstructure:"credentials_file" yaml:"credentials_file" json:"credentials_file"`

	// ProjectID is the Google Cloud Project ID (optional).
	ProjectID string `mapstructure:"project_id" yaml:"project_id" json:"project_id"`

	// Logger options for GCS operations.
	Logger struct {
		Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	} `mapstructure:"logger" yaml:"logger" json:"logger"`
}

// DefaultConfig returns safe default settings for GCS.
func DefaultConfig() Config {
	return Config{}
}
