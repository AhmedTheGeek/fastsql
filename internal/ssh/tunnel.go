// Package ssh provides SSH tunnel management for database connections.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Common errors for SSH tunnel operations.
var (
	ErrTunnelNotRunning  = errors.New("ssh tunnel is not running")
	ErrTunnelAlreadyOpen = errors.New("ssh tunnel is already open")
	ErrNoAuthMethod      = errors.New("no authentication method configured")
	ErrInvalidAuthMethod = errors.New("invalid authentication method")
)

// AuthMethod represents the SSH authentication method.
type AuthMethod string

const (
	AuthMethodKey      AuthMethod = "key"
	AuthMethodPassword AuthMethod = "password"
	AuthMethodAgent    AuthMethod = "agent"
)

// Config holds SSH tunnel configuration.
type Config struct {
	// SSH server settings
	Host string
	Port int
	User string

	// Authentication
	AuthMethod AuthMethod
	KeyPath    string
	Passphrase string
	Password   string

	// Tunnel settings
	LocalPort  int
	RemoteHost string
	RemotePort int

	// Health check settings
	HealthCheckInterval time.Duration
	ReconnectDelay      time.Duration
	MaxReconnectAttempts int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Port:                 22,
		HealthCheckInterval:  30 * time.Second,
		ReconnectDelay:       5 * time.Second,
		MaxReconnectAttempts: 3,
	}
}

// Status represents the current state of the tunnel.
type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
	StatusReconnecting
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Tunnel manages an SSH tunnel for database connections.
type Tunnel struct {
	config Config

	mu            sync.RWMutex
	status        Status
	lastError     error
	client        *ssh.Client
	listener      net.Listener
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	reconnectChan chan struct{}

	// Callbacks
	onStatusChange func(Status)
}

// New creates a new SSH tunnel with the given configuration.
func New(cfg Config) *Tunnel {
	return &Tunnel{
		config:        cfg,
		status:        StatusDisconnected,
		reconnectChan: make(chan struct{}, 1),
	}
}

// SetStatusCallback sets a callback function that's called when tunnel status changes.
func (t *Tunnel) SetStatusCallback(fn func(Status)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onStatusChange = fn
}

// Status returns the current tunnel status.
func (t *Tunnel) Status() Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// LastError returns the last error that occurred.
func (t *Tunnel) LastError() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastError
}

// LocalAddr returns the local address to connect to (for DB connection).
func (t *Tunnel) LocalAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", t.config.LocalPort)
}

// Start initiates the SSH tunnel connection.
func (t *Tunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.status == StatusConnected || t.status == StatusConnecting {
		t.mu.Unlock()
		return ErrTunnelAlreadyOpen
	}
	t.setStatusLocked(StatusConnecting)
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.mu.Unlock()

	if err := t.connect(); err != nil {
		t.setStatus(StatusError)
		t.setError(err)
		return err
	}

	t.setStatus(StatusConnected)

	// Start health check routine
	t.wg.Add(1)
	go t.healthCheckLoop()

	return nil
}

// Stop closes the SSH tunnel.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	if t.status == StatusDisconnected {
		t.mu.Unlock()
		return nil
	}

	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()

	// Wait for goroutines to finish
	t.wg.Wait()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		t.listener.Close()
		t.listener = nil
	}

	if t.client != nil {
		t.client.Close()
		t.client = nil
	}

	t.setStatusLocked(StatusDisconnected)
	return nil
}

// connect establishes the SSH connection and starts the tunnel.
func (t *Tunnel) connect() error {
	authMethods, err := t.buildAuthMethods()
	if err != nil {
		// Return as-is if it's already a known sentinel error
		if errors.Is(err, ErrInvalidAuthMethod) || errors.Is(err, ErrNoAuthMethod) {
			return err
		}
		return fmt.Errorf("failed to build auth methods: %w", err)
	}

	if len(authMethods) == 0 {
		return ErrNoAuthMethod
	}

	sshConfig := &ssh.ClientConfig{
		User:            t.config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add proper host key verification
		Timeout:         10 * time.Second,
	}

	serverAddr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	client, err := ssh.Dial("tcp", serverAddr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial SSH server %s: %w", serverAddr, err)
	}

	// Start local listener
	localAddr := fmt.Sprintf("127.0.0.1:%d", t.config.LocalPort)
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to listen on %s: %w", localAddr, err)
	}

	t.mu.Lock()
	t.client = client
	t.listener = listener
	t.mu.Unlock()

	// Start accepting connections
	t.wg.Add(1)
	go t.acceptLoop()

	return nil
}

// buildAuthMethods creates SSH authentication methods based on config.
func (t *Tunnel) buildAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	switch t.config.AuthMethod {
	case AuthMethodKey:
		method, err := t.keyAuthMethod()
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)

	case AuthMethodPassword:
		if t.config.Password == "" {
			return nil, errors.New("password is required for password authentication")
		}
		methods = append(methods, ssh.Password(t.config.Password))

	case AuthMethodAgent:
		method, err := t.agentAuthMethod()
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)

	default:
		return nil, fmt.Errorf("%w", ErrInvalidAuthMethod)
	}

	return methods, nil
}

// keyAuthMethod creates an SSH auth method from a private key file.
func (t *Tunnel) keyAuthMethod() (ssh.AuthMethod, error) {
	keyPath := t.config.KeyPath
	if keyPath == "" {
		// Default to ~/.ssh/id_rsa
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = home + "/.ssh/id_rsa"
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", keyPath, err)
	}

	var signer ssh.Signer
	if t.config.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(t.config.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// agentAuthMethod creates an SSH auth method using the SSH agent.
func (t *Tunnel) agentAuthMethod() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, errors.New("SSH_AUTH_SOCK not set; SSH agent not available")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// acceptLoop accepts incoming connections on the local listener.
func (t *Tunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		t.mu.RLock()
		listener := t.listener
		ctx := t.ctx
		t.mu.RUnlock()

		if listener == nil {
			return
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set accept deadline to allow periodic cancellation checks
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		localConn, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			// Listener closed or other error
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		t.wg.Add(1)
		go t.handleConnection(localConn)
	}
}

// handleConnection forwards a local connection through the SSH tunnel.
func (t *Tunnel) handleConnection(localConn net.Conn) {
	defer t.wg.Done()
	defer localConn.Close()

	t.mu.RLock()
	client := t.client
	ctx := t.ctx
	t.mu.RUnlock()

	if client == nil {
		return
	}

	remoteAddr := fmt.Sprintf("%s:%d", t.config.RemoteHost, t.config.RemotePort)
	remoteConn, err := client.Dial("tcp", remoteAddr)
	if err != nil {
		t.triggerReconnect()
		return
	}
	defer remoteConn.Close()

	// Bidirectional copy
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}

// healthCheckLoop periodically checks tunnel health and reconnects if needed.
func (t *Tunnel) healthCheckLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return

		case <-t.reconnectChan:
			t.handleReconnect()

		case <-ticker.C:
			if !t.isHealthy() {
				t.handleReconnect()
			}
		}
	}
}

// isHealthy checks if the SSH connection is still active.
func (t *Tunnel) isHealthy() bool {
	t.mu.RLock()
	client := t.client
	t.mu.RUnlock()

	if client == nil {
		return false
	}

	// Send a keepalive request
	_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

// triggerReconnect signals that a reconnection attempt is needed.
func (t *Tunnel) triggerReconnect() {
	select {
	case t.reconnectChan <- struct{}{}:
	default:
		// Reconnect already pending
	}
}

// handleReconnect attempts to reconnect the tunnel.
func (t *Tunnel) handleReconnect() {
	t.setStatus(StatusReconnecting)

	// Close existing connections
	t.mu.Lock()
	if t.listener != nil {
		t.listener.Close()
		t.listener = nil
	}
	if t.client != nil {
		t.client.Close()
		t.client = nil
	}
	t.mu.Unlock()

	attempts := 0
	maxAttempts := t.config.MaxReconnectAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	for attempts < maxAttempts {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		attempts++
		time.Sleep(t.config.ReconnectDelay)

		if err := t.connect(); err != nil {
			t.setError(err)
			continue
		}

		t.setStatus(StatusConnected)
		t.setError(nil)
		return
	}

	t.setStatus(StatusError)
	t.setError(fmt.Errorf("failed to reconnect after %d attempts", maxAttempts))
}

// setStatus updates the tunnel status and calls the callback.
func (t *Tunnel) setStatus(status Status) {
	t.mu.Lock()
	t.setStatusLocked(status)
	t.mu.Unlock()
}

// setStatusLocked updates status while already holding the lock.
func (t *Tunnel) setStatusLocked(status Status) {
	if t.status == status {
		return
	}
	t.status = status
	if t.onStatusChange != nil {
		go t.onStatusChange(status)
	}
}

// setError stores the last error.
func (t *Tunnel) setError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastError = err
}
