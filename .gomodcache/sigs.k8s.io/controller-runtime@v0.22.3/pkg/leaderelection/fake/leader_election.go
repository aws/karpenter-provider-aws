/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fake

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/leaderelection"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
)

// ControllableResourceLockInterface is an interface that extends resourcelock.Interface to be
// controllable.
type ControllableResourceLockInterface interface {
	resourcelock.Interface

	// BlockLeaderElection blocks the leader election process when called. It will not be unblocked
	// until UnblockLeaderElection is called.
	BlockLeaderElection()

	// UnblockLeaderElection unblocks the leader election.
	UnblockLeaderElection()
}

// NewResourceLock creates a new ResourceLock for use in testing
// leader election.
func NewResourceLock(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
	// Leader id, needs to be unique
	id, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	id = id + "_" + string(uuid.NewUUID())

	return &resourceLock{
		id: id,
		record: resourcelock.LeaderElectionRecord{
			HolderIdentity:       id,
			LeaseDurationSeconds: 1,
			AcquireTime:          metav1.NewTime(time.Now()),
			RenewTime:            metav1.NewTime(time.Now().Add(1 * time.Second)),
			LeaderTransitions:    1,
		},
	}, nil
}

var _ ControllableResourceLockInterface = &resourceLock{}

// resourceLock implements the ResourceLockInterface.
// By default returns that the current identity holds the lock.
type resourceLock struct {
	id     string
	record resourcelock.LeaderElectionRecord

	blockedLeaderElection atomic.Bool
}

// Get implements the ResourceLockInterface.
func (f *resourceLock) Get(ctx context.Context) (*resourcelock.LeaderElectionRecord, []byte, error) {
	recordBytes, err := json.Marshal(f.record)
	if err != nil {
		return nil, nil, err
	}
	return &f.record, recordBytes, nil
}

// Create implements the ResourceLockInterface.
func (f *resourceLock) Create(ctx context.Context, ler resourcelock.LeaderElectionRecord) error {
	if f.blockedLeaderElection.Load() {
		// If leader election is blocked, we do not allow creating a new record.
		return errors.New("leader election is blocked, cannot create new record")
	}

	f.record = ler
	return nil
}

// Update implements the ResourceLockInterface.
func (f *resourceLock) Update(ctx context.Context, ler resourcelock.LeaderElectionRecord) error {
	if f.blockedLeaderElection.Load() {
		// If leader election is blocked, we do not allow updating records
		return errors.New("leader election is blocked, cannot update record")
	}

	f.record = ler

	return nil
}

// RecordEvent implements the ResourceLockInterface.
func (f *resourceLock) RecordEvent(s string) {

}

// Identity implements the ResourceLockInterface.
func (f *resourceLock) Identity() string {
	return f.id
}

// Describe implements the ResourceLockInterface.
func (f *resourceLock) Describe() string {
	return f.id
}

// BlockLeaderElection blocks the leader election process when called.
func (f *resourceLock) BlockLeaderElection() {
	f.blockedLeaderElection.Store(true)
}

// UnblockLeaderElection blocks the leader election process when called.
func (f *resourceLock) UnblockLeaderElection() {
	f.blockedLeaderElection.Store(false)
}
