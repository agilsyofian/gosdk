package domain

// UserIdentity represents the domain entity required by AuthService to authenticate
// a user and issue tokens/sessions, completely decoupled from transport layers (gRPC/Protobuf).
type UserIdentity struct {
	UserID            string   `json:"userId"`
	Subject           string   `json:"subject"`
	Email             string   `json:"email"`
	PhoneNumber       string   `json:"phoneNumber"`
	FullName          string   `json:"fullName"`
	DisplayName       string   `json:"displayName"`
	AvatarURL         string   `json:"avatarUrl"`
	Locale            string   `json:"locale"`
	Timezone          string   `json:"timezone"`
	Status            string   `json:"status"`
	DevicesIdentifier string   `json:"devicesIdentifier"`
	Platform          string   `json:"platform"`
	DeviceName        string   `json:"deviceName"`
	PushToken         string   `json:"pushToken"`
	Roles             []string `json:"roles"`
	Permissions       []string `json:"permissions"`
	PasswordHash      string   `json:"passwordHash,omitempty"`
	BiometricEnabled  bool     `json:"biometricEnabled,omitempty"`
	BiometricHash     string   `json:"biometricHash,omitempty"`
}
