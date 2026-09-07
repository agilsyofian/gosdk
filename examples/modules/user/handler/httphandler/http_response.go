package httphandler

import "github.com/google/uuid"

type UserDetail struct {
	// user_profiles
	UserId      uuid.UUID `json:"userId"`
	Status      string    `json:"status"`
	Email       string    `json:"email"`
	PhoneNumber string    `json:"phoneNumber"`
	FullName    string    `json:"fullName"`
	DisplayName string    `json:"displayName"`
	AvatarUrl   string    `json:"avatarUrl"`
	Locale      string    `json:"locale"`
	Timezone    string    `json:"timezone"`

	// user_auths
	Subject          string `json:"subject"`
	BiometricEnabled bool   `json:"biometricEnabled"`

	// user_roles
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`

	// user_devices
	DevicesIdentifier string `json:"devicesIdentifier"`
	Platform          string `json:"platform"`
	DeviceName        string `json:"deviceName"`
	PushToken         string `json:"pushToken"`
}
