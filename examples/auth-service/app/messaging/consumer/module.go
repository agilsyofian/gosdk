package consumer

import (
	"context"

	"go.uber.org/fx"
)

// Kafka wires the example's Kafka consumers into the application
// lifecycle: fx.OnStart launches the per-topic goroutines once the rest
// of the app has finished starting, and fx.OnStop cancels them and
// waits for a clean exit when the app shuts down. See NewConsumer,
// Consumer.Start and Consumer.Stop.
//
// fx.Provide alone never calls NewConsumer — fx only builds providers
// that something else depends on. Nothing in this app takes an
// IConsumer as a parameter, so without fx.Invoke below, NewConsumer
// (and therefore the OnStart/OnStop hooks attached to it) would never
// run and the consumer goroutines would never start.
var Kafka = fx.Module("consumer-kafka",
	fx.Provide(
		fx.Annotate(
			NewConsumer,
			fx.OnStop(func(ctx context.Context, c IConsumer) error {
				return c.Stop(ctx)
			}),
		),
	),
	fx.Invoke(func(c IConsumer) {
		c.Start()
	}),
)
