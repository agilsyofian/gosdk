package user_event

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/auth/service"
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
	event := service.UserEvent{
		EventID:   "evt-1",
		EventType: service.UserEventCreated,
		UserID:    "user-1",
		Data:      []byte(`{"foo":"bar"}`),
	}
	assert.Panics(t, func() {
		_ = pub.PublishUserEvent(context.Background(), event)
	})
}
