//go:build container

package server

import (
	"musicbox/internal/config"
	"musicbox/internal/service"
	"musicbox/internal/service/process"
)

func newServiceManager(cfg *config.ManagerConfig) (service.Manager, error) {
	return process.New(cfg), nil
}
