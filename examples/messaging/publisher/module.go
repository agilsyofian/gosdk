package publisher

import "go.uber.org/fx"

var Kafka = fx.Module("publisher-kafka",
	fx.Provide(
		func(deps DependenciesPublisher) IPublisher {
			return NewPublisher(deps)
		},
	),
)
