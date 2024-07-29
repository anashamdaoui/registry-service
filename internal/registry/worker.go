package registry

import "time"

type Worker struct {
	Address         string
	IsHealthy       bool
	LastHealthCheck time.Time
}
