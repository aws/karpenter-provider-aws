package portforward

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Package variables for port-forwarding
var (
	portForwardCmd     *exec.Cmd
	portForwardActive  bool
	portForwardMutex   sync.Mutex
	portForwardCleanup sync.Once
)

// SetupPortForwarding establishes port-forwarding to the metrics endpoint
func SetupPortForwarding(namespace, service string, localPort, remotePort int) error {
	portForwardMutex.Lock()
	defer portForwardMutex.Unlock()

	// If port-forwarding is already active, do nothing
	if portForwardActive && portForwardCmd != nil && portForwardCmd.Process != nil {
		return nil
	}

	// Set up port-forwarding in the background
	log.Printf("Setting up port-forwarding: %s:%d -> localhost:%d", service, remotePort, localPort)
	portForwardCmd = exec.Command("kubectl", "port-forward", "-n", namespace,
		fmt.Sprintf("svc/%s", service),
		fmt.Sprintf("%d:%d", localPort, remotePort))

	if err := portForwardCmd.Start(); err != nil {
		return fmt.Errorf("error starting port-forward: %v", err)
	}

	// Register cleanup handler (only once)
	setupCleanupHandler()

	// Give port-forwarding a moment to establish
	time.Sleep(2 * time.Second)
	portForwardActive = true
	log.Printf("Port-forwarding established")

	return nil
}

// CleanupPortForwarding terminates the port-forwarding process
func CleanupPortForwarding() {
	portForwardMutex.Lock()
	defer portForwardMutex.Unlock()

	if portForwardActive && portForwardCmd != nil && portForwardCmd.Process != nil {
		log.Println("Cleaning up port-forwarding...")
		if err := portForwardCmd.Process.Kill(); err != nil {
			log.Printf("Error killing port-forward process: %v", err)
		}
		portForwardActive = false
		portForwardCmd = nil
	}
}

// setupCleanupHandler ensures port-forwarding is cleaned up when the program exits
func setupCleanupHandler() {
	portForwardCleanup.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			CleanupPortForwarding()
			os.Exit(0)
		}()
	})
}

// DefaultPortForward sets up port-forwarding with karpenter defaults
func DefaultPortForward() error {
	return SetupPortForwarding("kube-system", "karpenter", 8080, 8080)
}
