//go:build !linux

package daemon

var _ DaemonManager = &noopDaemonManager{}

type noopDaemonManager struct{}

func NewDaemonManager() (DaemonManager, error) {
	return &noopDaemonManager{}, nil
}

func (m *noopDaemonManager) StartDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) StopDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) RestartDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) GetDaemonStatus(name string) (DaemonStatus, error) {
	return DaemonStatusUnknown, nil
}

func (m *noopDaemonManager) EnableDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) DisableDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) Close() {}
