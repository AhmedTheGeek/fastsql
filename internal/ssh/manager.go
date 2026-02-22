package ssh

import (
	"context"
	"fmt"
	"sync"

	"github.com/AhmedTheGeek/fastsql/models"
)

// Manager handles SSH tunnels for multiple connections.
type Manager struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel // keyed by connection name/URL
}

// GlobalManager is the global SSH tunnel manager instance.
var GlobalManager = NewManager()

// NewManager creates a new SSH tunnel manager.
func NewManager() *Manager {
	return &Manager{
		tunnels: make(map[string]*Tunnel),
	}
}

// ConfigFromSSHConfig converts a models.SSHConfig to an ssh.Config.
func ConfigFromSSHConfig(cfg *models.SSHConfig) Config {
	sshCfg := DefaultConfig()

	sshCfg.Host = cfg.Host
	if cfg.Port > 0 {
		sshCfg.Port = cfg.Port
	}
	sshCfg.User = cfg.User

	switch cfg.AuthMethod {
	case "key":
		sshCfg.AuthMethod = AuthMethodKey
	case "password":
		sshCfg.AuthMethod = AuthMethodPassword
	case "agent":
		sshCfg.AuthMethod = AuthMethodAgent
	default:
		// Default to key auth
		sshCfg.AuthMethod = AuthMethodKey
	}

	sshCfg.KeyPath = cfg.KeyPath
	sshCfg.Passphrase = cfg.Passphrase
	sshCfg.Password = cfg.Password
	sshCfg.LocalPort = cfg.TunnelLocalPort
	sshCfg.RemoteHost = cfg.TunnelRemoteHost
	sshCfg.RemotePort = cfg.TunnelRemotePort

	return sshCfg
}

// StartTunnel creates and starts an SSH tunnel for a connection.
func (m *Manager) StartTunnel(ctx context.Context, connID string, sshCfg *models.SSHConfig) (*Tunnel, error) {
	if sshCfg == nil || !sshCfg.Enabled {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if tunnel already exists
	if existing, ok := m.tunnels[connID]; ok {
		if existing.Status() == StatusConnected {
			return existing, nil
		}
		// Clean up old tunnel
		existing.Stop()
		delete(m.tunnels, connID)
	}

	// Create and start new tunnel
	cfg := ConfigFromSSHConfig(sshCfg)
	tunnel := New(cfg)

	if err := tunnel.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	m.tunnels[connID] = tunnel
	return tunnel, nil
}

// StopTunnel stops and removes the tunnel for a connection.
func (m *Manager) StopTunnel(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[connID]
	if !ok {
		return nil
	}

	if err := tunnel.Stop(); err != nil {
		return err
	}

	delete(m.tunnels, connID)
	return nil
}

// GetTunnel returns the tunnel for a connection if it exists.
func (m *Manager) GetTunnel(connID string) *Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tunnels[connID]
}

// GetTunnelStatus returns the status of a tunnel for a connection.
func (m *Manager) GetTunnelStatus(connID string) Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, ok := m.tunnels[connID]
	if !ok {
		return StatusDisconnected
	}
	return tunnel.Status()
}

// StopAll stops all active tunnels.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, tunnel := range m.tunnels {
		tunnel.Stop()
		delete(m.tunnels, id)
	}
}

// ActiveTunnels returns the number of active tunnels.
func (m *Manager) ActiveTunnels() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tunnels)
}
