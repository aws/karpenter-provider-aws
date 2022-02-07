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

package node

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/utils/timespan"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/injectabletime"
)

// Expiration is a subreconciler that terminates nodes after a period of time.
type Expiration struct {
	kubeClient client.Client
}

// Reconcile reconciles the node
func (r *Expiration) Reconcile(ctx context.Context, provisioner *v1alpha5.Provisioner, node *v1.Node) (reconcile.Result, error) {
	// 1. Ignore node if not applicable
	if provisioner.Spec.TTLSecondsUntilExpired == nil {
		return reconcile.Result{}, nil
	}
	// 2. Trigger termination workflow if expired
	expirationTTL := time.Duration(ptr.Int64Value(provisioner.Spec.TTLSecondsUntilExpired)) * time.Second
	expirationTime := node.CreationTimestamp.Add(expirationTTL)
	now := injectabletime.Now()
	if now.After(expirationTime) {
		//2a. Trigger termination workflow if expirationTime falls in one of maintenance windows,
		if len(provisioner.Spec.MaintenanceWindows) != 0 {
			return r.reconcileMaintenanceWindows(ctx, provisioner, node, now, expirationTTL, expirationTime)
		}
		//2b. Trigger immediate termination if maintenance windows are not specified.
		return r.deleteNode(ctx, node, expirationTTL, expirationTime)
	}
	// 3. Backoff until expired
	return reconcile.Result{RequeueAfter: time.Until(expirationTime)}, nil
}

func (r *Expiration) reconcileMaintenanceWindows(ctx context.Context, provisioner *v1alpha5.Provisioner, node *v1.Node, now time.Time, expirationTTL time.Duration, expirationTime time.Time) (reconcile.Result, error) {
	var nextWindows []time.Duration
	for _, maintenanceWindow := range provisioner.Spec.MaintenanceWindows {
		currentTime := timespan.ParseTimeWithZoneInDuration(now, maintenanceWindow.TimeZone)
		for _, weekDay := range maintenanceWindow.WeekDays {
			startTime := timespan.ParseStringTimeInDuration(weekDay, maintenanceWindow.StartTime)
			endTime := startTime + timespan.ParseStringTimeInDuration("0", maintenanceWindow.Duration)

			if endTime >= timespan.WeekEnd {
				if timespan.WithInTimeRange(startTime, timespan.WeekEnd, currentTime) || timespan.WithInTimeRange(0, endTime-timespan.WeekEnd, currentTime) {
					return r.deleteNode(ctx, node, expirationTTL, expirationTime)
				}
			} else {
				if timespan.WithInTimeRange(startTime, endTime, currentTime) {
					return r.deleteNode(ctx, node, expirationTTL, expirationTime)
				}
			}

			if startTime > currentTime {
				nextWindows = append(nextWindows, startTime-currentTime)
			}
			if startTime < currentTime {
				nextWindows = append(nextWindows, (timespan.WeekEnd-currentTime)+startTime)
			}

		}
	}

	return reconcile.Result{RequeueAfter: timespan.FindMinTimeDuration(nextWindows)}, nil

}

func (r *Expiration) deleteNode(ctx context.Context, node *v1.Node, expirationTTL time.Duration, expirationTime time.Time) (reconcile.Result, error) {
	logging.FromContext(ctx).Infof("Triggering termination for expired node after %s (+%s)", expirationTTL, time.Since(expirationTime))
	if err := r.kubeClient.Delete(ctx, node); err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting node, %w", err)
	}
	return reconcile.Result{}, nil
}
