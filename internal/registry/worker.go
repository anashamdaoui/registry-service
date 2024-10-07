package registry

import "time"

type Worker struct {
	Host            string
	HTTPPort        int32
	GRPCPort        int32
	IsHealthy       bool
	LastHealthCheck time.Time
}
