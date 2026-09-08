package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/messaging/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"user-service/app/config"
)

type dummySubscriber struct {
	topic string
}

func (d *dummySubscriber) Topic() string {
	return d.topic
}

func (d *dummySubscriber) Handler() kafka.Handler {
	return func(ctx context.Context, msg *kafka.Message) error {
		return nil
	}
}

func TestConsumer_NewConsumer_EmptySubscribers(t *testing.T) {
	deps := DependenciesConsumer{
		AppConfig: config.AppConfig{
			AppName: "test-app",
		},
		Client:      nil,
		Subscribers: nil,
	}

	c, err := NewConsumer(deps)
	require.NoError(t, err)
	require.NotNil(t, c)

	c.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = c.Stop(ctx)
	assert.NoError(t, err)
}

func TestConsumer_NewConsumer_SkipsEmptyTopic(t *testing.T) {
	deps := DependenciesConsumer{
		AppConfig: config.AppConfig{
			AppName: "test-app",
		},
		Client: nil,
		Subscribers: []TopicSubscriber{
			&dummySubscriber{topic: ""},
		},
	}

	c, err := NewConsumer(deps)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, 0, len(c.(*Consumer).subscriptions))
}
