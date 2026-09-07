package domain

import (
	"github.com/agilsyofian/gosdk/security"
	"github.com/google/uuid"
)

// User is intentionally a fairly rich domain object — it mirrors the
// v_users Postgres view (repository/user.go), which joins four tables
// (profile, auth, roles/permissions, device). That's on purpose: it
// shows the SDK's mapping/session/RBAC patterns working against a
// realistic shape, not a toy one.
//
// If you're here to learn the SDK's patterns rather than this
// particular domain, the fields that matter are Email, Roles,
// Permissions, and Status — those are what flow into
// security.Principal.Claims and drive RequirePermission checks
// (server/http.go). BiometricEnabled/BiometricHash, DevicesIdentifier,
// Platform, DeviceName, and PushToken are domain richness from the
// underlying schema, not something the SDK itself cares about — treat
// them as "the SDK is fine with a User this wide," not as fields you
// need to understand before the SDK patterns make sense.
type User struct {
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
	PasswordHash     string `json:"passwordHash"`
	BiometricEnabled bool   `json:"biometricEnabled"`
	BiometricHash    string `json:"biometricHash"`

	// user_roles
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`

	// user_devices
	DevicesIdentifier string `json:"devicesIdentifier"`
	Platform          string `json:"platform"`
	DeviceName        string `json:"deviceName"`
	PushToken         string `json:"pushToken"`
}

func UserFromPrincipal(p *security.Principal) (User, error) {
	var user User
	if p == nil {
		return user, nil
	}

	userID, err := uuid.Parse(p.Subject)
	if err != nil {
		return user, err
	}

	user.UserId = userID
	user.Subject = p.Username
	user.Roles = append([]string(nil), p.Roles...)
	user.Permissions = append([]string(nil), p.Permissions...)

	user.Email = stringClaim(p, "email")
	user.FullName = stringClaim(p, "fullName")
	user.DisplayName = stringClaim(p, "displayName")
	user.Status = stringClaim(p, "status")
	user.PhoneNumber = stringClaim(p, "phoneNumber")
	user.AvatarUrl = stringClaim(p, "avatarUrl")
	user.Locale = stringClaim(p, "locale")
	user.Timezone = stringClaim(p, "timezone")
	user.DevicesIdentifier = stringClaim(p, "devicesIdentifier")
	user.Platform = stringClaim(p, "platform")
	user.DeviceName = stringClaim(p, "deviceName")
	user.PushToken = stringClaim(p, "pushToken")

	return user, nil
}

func stringClaim(p *security.Principal, name string) string {
	value, ok := p.Claim(name)
	if !ok {
		return ""
	}

	valueString, _ := value.(string)
	return valueString
}
