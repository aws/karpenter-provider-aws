//go:build linux

package daemon

import (
	"context"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"
)

var _ DaemonManager = &systemdDaemonManager{}

type systemdDaemonManager struct {
	conn *dbus.Conn
}

const (
	ModeReplace = "replace"
	TypeSymlink = "symlink"
	TypeUnlink  = "unlink"
)

func NewDaemonManager() (DaemonManager, error) {
	conn, err := dbus.NewWithContext(context.Background())
	if err != nil {
		return nil, err
	}
	return &systemdDaemonManager{
		conn: conn,
	}, nil
}

func (m *systemdDaemonManager) StartDaemon(name string) error {
	unitName := getServiceUnitName(name)
	_, err := m.conn.StartUnitContext(context.TODO(), unitName, ModeReplace, nil)
	return err
}

func (m *systemdDaemonManager) StopDaemon(name string) error {
	unitName := getServiceUnitName(name)
	_, err := m.conn.StopUnitContext(context.TODO(), unitName, ModeReplace, nil)
	return err
}

func (m *systemdDaemonManager) RestartDaemon(name string) error {
	unitName := getServiceUnitName(name)
	_, err := m.conn.RestartUnitContext(context.TODO(), unitName, ModeReplace, nil)
	return err
}

func (m *systemdDaemonManager) GetDaemonStatus(name string) (DaemonStatus, error) {
	unitName := getServiceUnitName(name)
	status, err := m.conn.GetUnitPropertyContext(context.TODO(), unitName, "ActiveState")
	if err != nil {
		return DaemonStatusUnknown, err
	}
	switch status.Value.String() {
	case "active":
		return DaemonStatusRunning, nil
	case "inactive":
		return DaemonStatusStopped, nil
	default:
		return DaemonStatusUnknown, nil
	}
}

func (m *systemdDaemonManager) EnableDaemon(name string) error {
	unitName := getServiceUnitName(name)
	_, changes, err := m.conn.EnableUnitFilesContext(context.TODO(), []string{unitName}, false, false)
	if err != nil {
		return err
	}
	if len(changes) != 1 {
		return fmt.Errorf("unexpected number of unit file changes: %d", len(changes))
	}
	if changes[0].Type != TypeSymlink {
		return fmt.Errorf("unexpected unit file change type: %s", changes[0].Type)
	}
	return nil
}

func (m *systemdDaemonManager) DisableDaemon(name string) error {
	unitName := getServiceUnitName(name)
	changes, err := m.conn.DisableUnitFilesContext(context.TODO(), []string{unitName}, false)
	if err != nil {
		return err
	}
	if len(changes) != 1 {
		return fmt.Errorf("unexpected number of unit file changes: %d", len(changes))
	}
	if changes[0].Type != TypeUnlink {
		return fmt.Errorf("unexpected unit file change type: %s", changes[0].Type)
	}
	return nil
}

func (m *systemdDaemonManager) Close() {
	m.conn.Close()
}

func getServiceUnitName(name string) string {
	return fmt.Sprintf("%s.service", name)
}
