package user_event

import (
	"context"
	"testing"

	"auth-service/app/config"
	"auth-service/module/auth/port"

	"github.com/stretchr/testify/assert"
)

func TestUserEventPublisher_NilClient(t *testing.T) {
	pub, err := NewUserEventPublisher(nil, config.AppConfig{})
	assert.Error(t, err)
	assert.Nil(t, pub)
}

func TestUserEventPublisher_MarshalError(t *testing.T) {
	pub := &UserEventPublisher{producer: nil}
	// Event with normal data should attempt publish (and panic or fail because producer is nil)
	event := port.UserEvent{
		EventID:   "evt-1",
		EventType: port.UserEventCreated,
		UserID:    "user-1",
		Data:      []byte(`{"foo":"bar"}`),
	}
	assert.Panics(t, func() {
		_ = pub.PublishUserEvent(context.Background(), event)
	})
}
