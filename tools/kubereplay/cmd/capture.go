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

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/cloudwatch"
	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/format"
	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/parser"
	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/sanitizer"
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture workload events from EKS audit logs",
	RunE:  runCapture,
}

var (
	captureOutput   string
	captureDuration time.Duration
)

func init() {
	captureCmd.Flags().StringVarP(&captureOutput, "output", "o", "replay.json", "Output file")
	captureCmd.Flags().DurationVarP(&captureDuration, "duration", "d", time.Hour, "Duration to capture")
}

func runCapture(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	endTime := time.Now()
	startTime := endTime.Add(-captureDuration)

	cluster, err := clusterFromKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to detect cluster from kubeconfig: %w", err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	cwClient := cloudwatch.NewClient(cloudwatchlogs.NewFromConfig(cfg), cluster)
	p := parser.NewParser()
	san := sanitizer.New()
	replayLog := format.NewReplayLog(cluster)

	fmt.Printf("Capturing from %s (%s to %s)\n", cwClient.LogGroup, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	eventCh, errCh := cwClient.StreamEvents(ctx, cloudwatch.FetchOptions{
		StartTime: startTime,
		EndTime:   endTime,
	})

	var deploymentCount, jobCount, scaleCount, deleteCount, totalEvents int
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	// Store pending events to correlate after all events processed (CloudWatch returns events out of order)
	type pendingEvent struct {
		originalKey    string
		timestamp      time.Time
		replicas       int32     // only for scale events
		isDelete       bool      // deployment delete
		isJobComplete  bool      // job completion
		completionTime time.Time // for job completions
	}
	var pendingEvents []pendingEvent
	// Track original job key -> sanitized job key for duration correlation
	jobKeyMapping := make(map[string]string)
loop:
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				select {
				case err := <-errCh:
					if err != nil {
						return err
					}
				default:
				}
				break loop
			}

			totalEvents++
			if isTTY {
				fmt.Printf("\r  %s %d/%d/%d/%d (deploy/job/scale/del)\033[K", spinner[totalEvents%len(spinner)], deploymentCount, jobCount, scaleCount, deleteCount)
			}

			result, err := p.ParseEvent(*event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				continue
			}
			if result == nil {
				continue
			}

			// Sanitize and add to log
			if result.Deployment != nil {
				sanitized := san.SanitizeDeployment(result.Deployment)
				replayLog.AddDeploymentCreate(sanitized, result.Timestamp)
				deploymentCount++
			} else if result.Job != nil {
				originalKey := result.Job.Namespace + "/" + result.Job.Name
				sanitized := san.SanitizeJob(result.Job)
				sanitizedKey := sanitized.Namespace + "/" + sanitized.Name
				jobKeyMapping[originalKey] = sanitizedKey
				replayLog.AddJobCreate(sanitized, result.Timestamp)
				jobCount++
			} else if result.JobCompleteEvent != nil {
				// Store completion for later correlation (events may arrive out of order)
				originalKey := result.JobCompleteEvent.Namespace + "/" + result.JobCompleteEvent.Name
				pendingEvents = append(pendingEvents, pendingEvent{
					originalKey:    originalKey,
					timestamp:      result.Timestamp,
					isJobComplete:  true,
					completionTime: result.JobCompleteEvent.CompletionTime,
				})
			} else if result.ScaleEvent != nil {
				// Store scale for later correlation (events may arrive out of order)
				originalKey := result.ScaleEvent.Namespace + "/" + result.ScaleEvent.Name
				pendingEvents = append(pendingEvents, pendingEvent{
					originalKey: originalKey,
					timestamp:   result.Timestamp,
					replicas:    result.ScaleEvent.Replicas,
				})
				scaleCount++ // Show pending count in progress
			} else if result.DeleteEvent != nil {
				// Store delete for later correlation (events may arrive out of order)
				originalKey := result.DeleteEvent.Namespace + "/" + result.DeleteEvent.Name
				pendingEvents = append(pendingEvents, pendingEvent{
					originalKey: originalKey,
					timestamp:   result.Timestamp,
					isDelete:    true,
				})
				deleteCount++ // Show pending count in progress
			}

		case err := <-errCh:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if isTTY {
		fmt.Println() // Clear progress line
	}

	// Correlate pending events with sanitized workloads
	// Reset counts since we only want to count events that match known workloads
	scaleCount, deleteCount = 0, 0
	var jobsWithDuration int
	for _, ev := range pendingEvents {
		if ev.isJobComplete {
			// Correlate job completion with job create to calculate duration
			sanitizedKey, ok := jobKeyMapping[ev.originalKey]
			if !ok {
				continue
			}
			// Get creation time from parser
			createTime, ok := p.GetJobCreateTime(ev.originalKey)
			if !ok {
				continue
			}
			duration := ev.completionTime.Sub(createTime)
			if duration > 0 {
				replayLog.SetJobDuration(sanitizedKey, duration)
				jobsWithDuration++
			}
		} else {
			// Deployment scale/delete events
			sanitizedKey, ok := san.GetSanitizedKey(ev.originalKey)
			if !ok {
				continue
			}
			parts := strings.SplitN(sanitizedKey, "/", 2)
			if ev.isDelete {
				replayLog.AddDeploymentDelete(parts[0], parts[1], ev.timestamp)
				deleteCount++
			} else {
				replayLog.AddDeploymentScale(parts[0], parts[1], ev.replicas, ev.timestamp)
				scaleCount++
			}
		}
	}

	if err := replayLog.WriteToFile(captureOutput); err != nil {
		return err
	}

	fmt.Printf("Captured %d deployments, %d jobs (%d with duration), %d scale, %d delete to %s\n",
		deploymentCount, jobCount, jobsWithDuration, scaleCount, deleteCount, captureOutput)
	return nil
}

func clusterFromKubeconfig() (string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return "", err
	}

	contextName := cfg.CurrentContext
	if contextName == "" {
		return "", fmt.Errorf("no current context")
	}

	context, ok := cfg.Contexts[contextName]
	if !ok {
		return "", fmt.Errorf("context %q not found", contextName)
	}

	// Try to extract from ARN pattern (arn:aws:eks:region:account:cluster/name)
	arnPattern := regexp.MustCompile(`cluster/([^/]+)$`)
	if matches := arnPattern.FindStringSubmatch(contextName); len(matches) > 1 {
		return matches[1], nil
	}
	if matches := arnPattern.FindStringSubmatch(context.Cluster); len(matches) > 1 {
		return matches[1], nil
	}

	return context.Cluster, nil
}
