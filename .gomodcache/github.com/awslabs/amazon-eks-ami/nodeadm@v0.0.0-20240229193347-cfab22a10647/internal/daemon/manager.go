package daemon

type DaemonStatus string

const (
	DaemonStatusRunning DaemonStatus = "running"
	DaemonStatusStopped DaemonStatus = "stopped"
	DaemonStatusUnknown DaemonStatus = "unknown"
)

type DaemonManager interface {
	// StartDaemon starts the daemon with the given name.
	// If the daemon is already running, this is a no-op.
	StartDaemon(name string) error
	// StopDaemon stops the daemon with the given name.
	// If the daemon is not running, this is a no-op.
	StopDaemon(name string) error
	// RestartDaemon restarts the daemon with the given name.
	// If the daemon is not running, it will be started.
	RestartDaemon(name string) error
	// GetDaemonStatus returns the status of the daemon with the given name.
	GetDaemonStatus(name string) (DaemonStatus, error)
	// EnableDaemon enables the daemon with the given name.
	// If the daemon is already enabled, this is a no-op.
	EnableDaemon(name string) error
	// DisableDaemon disables the daemon with the given name.
	// If the daemon is not enabled, this is a no-op.
	DisableDaemon(name string) error
	// Close cleans up any underlying resources used by the daemon manager.
	Close()
}
