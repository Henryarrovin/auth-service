package kafka_logger_pipeline

import "github.com/google/wire"

var providerSet = wire.NewSet(NewKafkaCoreWithLogger)
