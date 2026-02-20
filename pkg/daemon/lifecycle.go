package daemon

import (
	"context"
	"log/slog"
	"time"

	"thoreinstein.com/rig/pkg/plugin"
)

// Lifecycle manages the idle timeouts for the daemon and its plugins.
type Lifecycle struct {
	manager           *plugin.Manager
	server            *DaemonServer
	pluginIdleTimeout time.Duration
	daemonIdleTimeout time.Duration
	logger            *slog.Logger
	shutdown          chan struct{}
}

func NewLifecycle(m *plugin.Manager, s *DaemonServer, pluginIdle, daemonIdle time.Duration, logger *slog.Logger) *Lifecycle {
	if pluginIdle == 0 {
		pluginIdle = 5 * time.Minute
	}
	if daemonIdle == 0 {
		daemonIdle = 15 * time.Minute
	}
	return &Lifecycle{
		manager:           m,
		server:            s,
		pluginIdleTimeout: pluginIdle,
		daemonIdleTimeout: daemonIdle,
		logger:            logger,
		shutdown:          make(chan struct{}),
	}
}

// Run starts the background reaper goroutine.
func (l *Lifecycle) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.shutdown:
			return
		case <-ticker.C:
			l.checkIdle()
		}
	}
}

func (l *Lifecycle) checkIdle() {
	// 1. Check for daemon idle timeout
	l.server.mu.Lock()
	active := l.server.activeSessions
	l.server.mu.Unlock()

	if active == 0 && time.Since(l.server.startTime) > l.daemonIdleTimeout {
		if l.logger != nil {
			l.logger.Info("Daemon reached idle timeout, shutting down")
		}
		// Signal shutdown to the main process
		l.Stop()
	}

	// 2. Check for plugin idle timeouts
	plugins := l.manager.ListPlugins()
	for _, p := range plugins {
		if time.Since(p.LastUsedTime()) > l.pluginIdleTimeout {
			if l.logger != nil {
				l.logger.Info("Plugin reached idle timeout, stopping", "plugin", p.Name)
			}
			_ = l.manager.StopPlugin(p.Name)
		}
	}
}

func (l *Lifecycle) Stop() {
	close(l.shutdown)
}
