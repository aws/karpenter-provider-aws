/*
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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/config"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/events"
)

type ControllerGetterFunc func(context.Context, *ControllerOptions) []Controller

type ControllerOptions struct {
	Config     config.Config
	Cluster    *state.Cluster
	KubeClient client.Client
	Recorder   events.Recorder
	Clock      clock.Clock

	StartAsync <-chan struct{}
}

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(context.Context, reconcile.Request) (reconcile.Result, error)
	// Register will register the controller with the manager
	Register(context.Context, manager.Manager) error
}

type ignoreDebugEventsSink struct {
	name string
	sink logr.LogSink
}

func (i ignoreDebugEventsSink) Init(ri logr.RuntimeInfo) {
	i.sink.Init(ri)
}
func (i ignoreDebugEventsSink) Enabled(level int) bool { return i.sink.Enabled(level) }
func (i ignoreDebugEventsSink) Info(level int, msg string, keysAndValues ...interface{}) {
	// ignore debug "events" logs
	if level == 1 && i.name == "events" {
		return
	}
	i.sink.Info(level, msg, keysAndValues...)
}
func (i ignoreDebugEventsSink) Error(err error, msg string, keysAndValues ...interface{}) {
	i.sink.Error(err, msg, keysAndValues...)
}
func (i ignoreDebugEventsSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return i.sink.WithValues(keysAndValues...)
}
func (i ignoreDebugEventsSink) WithName(name string) logr.LogSink {
	return &ignoreDebugEventsSink{name: name, sink: i.sink.WithName(name)}
}
