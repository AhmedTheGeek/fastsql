package ssh

import (
	"testing"

	"github.com/AhmedTheGeek/fastsql/models"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.ActiveTunnels() != 0 {
		t.Errorf("NewManager().ActiveTunnels() = %d, want 0", m.ActiveTunnels())
	}
}

func TestGlobalManagerExists(t *testing.T) {
	if GlobalManager == nil {
		t.Error("GlobalManager is nil")
	}
}

func TestConfigFromSSHConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *models.SSHConfig
		expected Config
	}{
		{
			name: "key auth",
			input: &models.SSHConfig{
				Enabled:          true,
				Host:             "example.com",
				Port:             2222,
				User:             "admin",
				AuthMethod:       "key",
				KeyPath:          "/home/user/.ssh/id_ed25519",
				Passphrase:       "secret",
				TunnelLocalPort:  13306,
				TunnelRemoteHost: "db.internal",
				TunnelRemotePort: 3306,
			},
			expected: Config{
				Host:                 "example.com",
				Port:                 2222,
				User:                 "admin",
				AuthMethod:           AuthMethodKey,
				KeyPath:              "/home/user/.ssh/id_ed25519",
				Passphrase:           "secret",
				LocalPort:            13306,
				RemoteHost:           "db.internal",
				RemotePort:           3306,
				HealthCheckInterval:  DefaultConfig().HealthCheckInterval,
				ReconnectDelay:       DefaultConfig().ReconnectDelay,
				MaxReconnectAttempts: DefaultConfig().MaxReconnectAttempts,
			},
		},
		{
			name: "password auth",
			input: &models.SSHConfig{
				Enabled:          true,
				Host:             "bastion.example.com",
				Port:             22,
				User:             "deploy",
				AuthMethod:       "password",
				Password:         "hunter2",
				TunnelLocalPort:  15432,
				TunnelRemoteHost: "postgres.internal",
				TunnelRemotePort: 5432,
			},
			expected: Config{
				Host:                 "bastion.example.com",
				Port:                 22,
				User:                 "deploy",
				AuthMethod:           AuthMethodPassword,
				Password:             "hunter2",
				LocalPort:            15432,
				RemoteHost:           "postgres.internal",
				RemotePort:           5432,
				HealthCheckInterval:  DefaultConfig().HealthCheckInterval,
				ReconnectDelay:       DefaultConfig().ReconnectDelay,
				MaxReconnectAttempts: DefaultConfig().MaxReconnectAttempts,
			},
		},
		{
			name: "agent auth",
			input: &models.SSHConfig{
				Enabled:          true,
				Host:             "jump.example.com",
				Port:             22,
				User:             "jump",
				AuthMethod:       "agent",
				TunnelLocalPort:  11433,
				TunnelRemoteHost: "127.0.0.1",
				TunnelRemotePort: 1433,
			},
			expected: Config{
				Host:                 "jump.example.com",
				Port:                 22,
				User:                 "jump",
				AuthMethod:           AuthMethodAgent,
				LocalPort:            11433,
				RemoteHost:           "127.0.0.1",
				RemotePort:           1433,
				HealthCheckInterval:  DefaultConfig().HealthCheckInterval,
				ReconnectDelay:       DefaultConfig().ReconnectDelay,
				MaxReconnectAttempts: DefaultConfig().MaxReconnectAttempts,
			},
		},
		{
			name: "default auth method",
			input: &models.SSHConfig{
				Enabled:          true,
				Host:             "server.com",
				Port:             22,
				User:             "user",
				AuthMethod:       "", // empty should default to key
				TunnelLocalPort:  13307,
				TunnelRemoteHost: "localhost",
				TunnelRemotePort: 3306,
			},
			expected: Config{
				Host:                 "server.com",
				Port:                 22,
				User:                 "user",
				AuthMethod:           AuthMethodKey,
				LocalPort:            13307,
				RemoteHost:           "localhost",
				RemotePort:           3306,
				HealthCheckInterval:  DefaultConfig().HealthCheckInterval,
				ReconnectDelay:       DefaultConfig().ReconnectDelay,
				MaxReconnectAttempts: DefaultConfig().MaxReconnectAttempts,
			},
		},
		{
			name: "default port",
			input: &models.SSHConfig{
				Enabled:          true,
				Host:             "server.com",
				Port:             0, // should use default 22
				User:             "user",
				AuthMethod:       "key",
				TunnelLocalPort:  13308,
				TunnelRemoteHost: "localhost",
				TunnelRemotePort: 3306,
			},
			expected: Config{
				Host:                 "server.com",
				Port:                 22, // default
				User:                 "user",
				AuthMethod:           AuthMethodKey,
				LocalPort:            13308,
				RemoteHost:           "localhost",
				RemotePort:           3306,
				HealthCheckInterval:  DefaultConfig().HealthCheckInterval,
				ReconnectDelay:       DefaultConfig().ReconnectDelay,
				MaxReconnectAttempts: DefaultConfig().MaxReconnectAttempts,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConfigFromSSHConfig(tt.input)

			if got.Host != tt.expected.Host {
				t.Errorf("Host = %q, want %q", got.Host, tt.expected.Host)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %d, want %d", got.Port, tt.expected.Port)
			}
			if got.User != tt.expected.User {
				t.Errorf("User = %q, want %q", got.User, tt.expected.User)
			}
			if got.AuthMethod != tt.expected.AuthMethod {
				t.Errorf("AuthMethod = %q, want %q", got.AuthMethod, tt.expected.AuthMethod)
			}
			if got.KeyPath != tt.expected.KeyPath {
				t.Errorf("KeyPath = %q, want %q", got.KeyPath, tt.expected.KeyPath)
			}
			if got.Passphrase != tt.expected.Passphrase {
				t.Errorf("Passphrase = %q, want %q", got.Passphrase, tt.expected.Passphrase)
			}
			if got.Password != tt.expected.Password {
				t.Errorf("Password = %q, want %q", got.Password, tt.expected.Password)
			}
			if got.LocalPort != tt.expected.LocalPort {
				t.Errorf("LocalPort = %d, want %d", got.LocalPort, tt.expected.LocalPort)
			}
			if got.RemoteHost != tt.expected.RemoteHost {
				t.Errorf("RemoteHost = %q, want %q", got.RemoteHost, tt.expected.RemoteHost)
			}
			if got.RemotePort != tt.expected.RemotePort {
				t.Errorf("RemotePort = %d, want %d", got.RemotePort, tt.expected.RemotePort)
			}
		})
	}
}

func TestManagerGetTunnelNotFound(t *testing.T) {
	m := NewManager()
	tunnel := m.GetTunnel("nonexistent")
	if tunnel != nil {
		t.Error("GetTunnel() should return nil for nonexistent connection")
	}
}

func TestManagerGetTunnelStatusNotFound(t *testing.T) {
	m := NewManager()
	status := m.GetTunnelStatus("nonexistent")
	if status != StatusDisconnected {
		t.Errorf("GetTunnelStatus() = %v, want StatusDisconnected", status)
	}
}

func TestManagerStopTunnelNotFound(t *testing.T) {
	m := NewManager()
	err := m.StopTunnel("nonexistent")
	if err != nil {
		t.Errorf("StopTunnel() returned error for nonexistent: %v", err)
	}
}

func TestManagerStopAll(t *testing.T) {
	m := NewManager()
	// Should not panic even when empty
	m.StopAll()

	if m.ActiveTunnels() != 0 {
		t.Error("StopAll() should leave no active tunnels")
	}
}

func TestManagerStartTunnelNilConfig(t *testing.T) {
	m := NewManager()

	tunnel, err := m.StartTunnel(nil, "test", nil)
	if err != nil {
		t.Errorf("StartTunnel() with nil config returned error: %v", err)
	}
	if tunnel != nil {
		t.Error("StartTunnel() with nil config should return nil tunnel")
	}
}

func TestManagerStartTunnelDisabled(t *testing.T) {
	m := NewManager()

	cfg := &models.SSHConfig{
		Enabled: false,
		Host:    "example.com",
	}

	tunnel, err := m.StartTunnel(nil, "test", cfg)
	if err != nil {
		t.Errorf("StartTunnel() with disabled config returned error: %v", err)
	}
	if tunnel != nil {
		t.Error("StartTunnel() with disabled config should return nil tunnel")
	}
}
