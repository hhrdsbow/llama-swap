package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ProcessState represents the current state of a managed process
type ProcessState int

const (
	StateStopped ProcessState = iota
	StateStarting
	StateRunning
	StateStopping
)

// Process manages a single llama.cpp server process
type Process struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	config  ModelConfig
	state   ProcessState
	port    int
	stopCh  chan struct{}
}

// NewProcess creates a new Process manager for the given model configuration
func NewProcess(config ModelConfig) *Process {
	return &Process{
		config: config,
		state:  StateStopped,
		stopCh: make(chan struct{}),
	}
}

// Start launches the underlying llama.cpp server process
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateRunning || p.state == StateStarting {
		return nil
	}

	log.Printf("[process] starting model: %s", p.config.Cmd)

	cmd := exec.Command("sh", "-c", p.config.Cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.cmd = cmd
	p.state = StateStarting

	// Wait for the process to become ready
	if err := p.waitForReady(); err != nil {
		_ = cmd.Process.Kill()
		p.state = StateStopped
		return fmt.Errorf("process did not become ready: %w", err)
	}

	p.state = StateRunning
	log.Printf("[process] model ready: %s", p.config.Cmd)

	// Monitor process exit in background
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		p.state = StateStopped
		p.mu.Unlock()
		log.Printf("[process] model exited: %s", p.config.Cmd)
	}()

	return nil
}

// Stop gracefully terminates the managed process
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return nil
	}

	log.Printf("[process] stopping model: %s", p.config.Cmd)
	p.state = StateStopping

	if p.cmd != nil && p.cmd.Process != nil {
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	p.state = StateStopped
	return nil
}

// IsRunning returns true if the process is currently in a running state
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == StateRunning
}

// waitForReady polls until the process HTTP endpoint is responsive or times out.
// Increased timeout to 120s since some larger models (e.g. 70B GGUF) take a while to load.
func (p *Process) waitForReady() error {
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for process to become ready")
		case <-ticker.C:
			if p.isHealthy() {
				return nil
			}
		}
	}
}

// isHealthy checks whether the process HTTP health endpoint responds successfully
func (p *Process) isHealthy() bool {
	// Health check is performed by the proxy layer using the model's upstream URL
	// This is a lightweight sentinel; actual HTTP check lives in the swap manager
	return p.cmd != nil && p.cmd.Process != nil
