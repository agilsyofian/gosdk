package domain

// Login represents the full authentication command payload processed by the service.
type Login struct {
	Username         string
	Password         string
	DeviceIdentifier string
}

// RefreshTokenCmd represents the command to issue a new token pair from an existing refresh token.
type RefreshTokenCmd struct {
	RefreshToken string
}

// TokenRefresh is an alias for RefreshTokenCmd for backward compatibility.
type TokenRefresh = RefreshTokenCmd
