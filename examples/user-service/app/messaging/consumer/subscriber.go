package consumer

import (
	"github.com/agilsyofian/gosdk/messaging/kafka"
	"go.uber.org/fx"
)

// TopicSubscriber is implemented by any business module that consumes messages from a Kafka topic.
type TopicSubscriber interface {
	Topic() string
	GroupID() string
	Concurrency() int
	AutoOffsetReset() string
	Handler() kafka.Handler
}

// AsTopicSubscriber annotates a constructor or instance as a TopicSubscriber in the "kafka_subscribers" fx group.
func AsTopicSubscriber(target any) any {
	return fx.Annotate(
		target,
		fx.As(new(TopicSubscriber)),
		fx.ResultTags(`group:"kafka_subscribers"`),
	)
}
