package ssh

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTunnelStatus(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusDisconnected, "disconnected"},
		{StatusConnecting, "connecting"},
		{StatusConnected, "connected"},
		{StatusReconnecting, "reconnecting"},
		{StatusError, "error"},
		{Status(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.expected {
			t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 22 {
		t.Errorf("DefaultConfig().Port = %d, want 22", cfg.Port)
	}

	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("DefaultConfig().HealthCheckInterval = %v, want 30s", cfg.HealthCheckInterval)
	}

	if cfg.ReconnectDelay != 5*time.Second {
		t.Errorf("DefaultConfig().ReconnectDelay = %v, want 5s", cfg.ReconnectDelay)
	}

	if cfg.MaxReconnectAttempts != 3 {
		t.Errorf("DefaultConfig().MaxReconnectAttempts = %d, want 3", cfg.MaxReconnectAttempts)
	}
}

func TestNewTunnel(t *testing.T) {
	cfg := Config{
		Host:       "example.com",
		Port:       22,
		User:       "testuser",
		AuthMethod: AuthMethodKey,
		LocalPort:  13306,
		RemoteHost: "localhost",
		RemotePort: 3306,
	}

	tunnel := New(cfg)

	if tunnel == nil {
		t.Fatal("New() returned nil")
	}

	if tunnel.Status() != StatusDisconnected {
		t.Errorf("New tunnel status = %v, want StatusDisconnected", tunnel.Status())
	}

	if tunnel.LocalAddr() != "127.0.0.1:13306" {
		t.Errorf("LocalAddr() = %q, want %q", tunnel.LocalAddr(), "127.0.0.1:13306")
	}
}

func TestTunnelStatusCallback(t *testing.T) {
	cfg := DefaultConfig()
	tunnel := New(cfg)

	var callbackStatus Status
	callbackCalled := make(chan struct{}, 1)

	tunnel.SetStatusCallback(func(s Status) {
		callbackStatus = s
		select {
		case callbackCalled <- struct{}{}:
		default:
		}
	})

	// Trigger a status change
	tunnel.setStatus(StatusConnecting)

	select {
	case <-callbackCalled:
		if callbackStatus != StatusConnecting {
			t.Errorf("Callback received status %v, want StatusConnecting", callbackStatus)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Status callback was not called")
	}
}

func TestTunnelLastError(t *testing.T) {
	tunnel := New(DefaultConfig())

	// Initially no error
	if tunnel.LastError() != nil {
		t.Error("New tunnel should have no error")
	}
}

func TestTunnelStopWhenNotRunning(t *testing.T) {
	tunnel := New(DefaultConfig())

	// Stop should not error when not running
	err := tunnel.Stop()
	if err != nil {
		t.Errorf("Stop() on non-running tunnel returned error: %v", err)
	}
}

func TestTunnelStartWithInvalidAuth(t *testing.T) {
	cfg := Config{
		Host:       "localhost",
		Port:       22,
		User:       "testuser",
		AuthMethod: AuthMethod("invalid"),
		LocalPort:  19999,
		RemoteHost: "localhost",
		RemotePort: 3306,
	}

	tunnel := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tunnel.Start(ctx)
	if err == nil {
		tunnel.Stop()
		t.Error("Start() with invalid auth method should return error")
	}

	if !errors.Is(err, ErrInvalidAuthMethod) {
		t.Errorf("Start() error = %v, want ErrInvalidAuthMethod", err)
	}
}

func TestTunnelStartWithEmptyPassword(t *testing.T) {
	cfg := Config{
		Host:       "localhost",
		Port:       22,
		User:       "testuser",
		AuthMethod: AuthMethodPassword,
		Password:   "", // Empty password
		LocalPort:  19998,
		RemoteHost: "localhost",
		RemotePort: 3306,
	}

	tunnel := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tunnel.Start(ctx)
	if err == nil {
		tunnel.Stop()
		t.Error("Start() with empty password should return error")
	}
}

func TestAuthMethodConstants(t *testing.T) {
	tests := []struct {
		method   AuthMethod
		expected string
	}{
		{AuthMethodKey, "key"},
		{AuthMethodPassword, "password"},
		{AuthMethodAgent, "agent"},
	}

	for _, tt := range tests {
		if string(tt.method) != tt.expected {
			t.Errorf("AuthMethod constant = %q, want %q", tt.method, tt.expected)
		}
	}
}
