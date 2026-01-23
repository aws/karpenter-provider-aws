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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/format"
	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/replay"
	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/sanitizer"
)

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay workload events against a cluster",
	RunE:  runReplay,
}

var (
	replayFile    string
	replayDryRun  bool
	replaySpeed   float64
	replayTimeout time.Duration
)

func init() {
	replayCmd.Flags().StringVarP(&replayFile, "file", "f", "", "Replay log file (required)")
	replayCmd.Flags().BoolVar(&replayDryRun, "dry-run", false, "Simulate replay with timing output, no workloads created")
	replayCmd.Flags().Float64Var(&replaySpeed, "speed", 1.0, "Time dilation factor (e.g., 24 = 24x faster, 24h replays in 1h)")
	replayCmd.Flags().DurationVar(&replayTimeout, "timeout", 10*time.Minute, "Max time to wait for stabilization (0 to disable)")
	_ = replayCmd.MarkFlagRequired("file")
}

func runReplay(cmd *cobra.Command, args []string) error {
	// Validate speed parameter
	if replaySpeed <= 0 {
		return fmt.Errorf("--speed must be greater than 0, got %v", replaySpeed)
	}

	replayLog, err := format.ReadFromFile(replayFile)
	if err != nil {
		return err
	}

	// Validate replay log
	if err := validateReplayLog(replayLog); err != nil {
		return fmt.Errorf("invalid replay log: %w", err)
	}

	summary := eventSummary(replayLog)
	fmt.Printf("Loaded %s from %s\n", summary, replayFile)

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Handle interrupt in background
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted")
		cancel()
	}()

	// Dry-run mode: simulate timing without creating workloads
	if replayDryRun {
		engine := replay.NewEngine(nil, sanitizer.Namespace)
		_, err := engine.RunTimed(ctx, replayLog, replaySpeed, true)
		return err
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return err
	}
	kubeClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return err
	}

	engine := replay.NewEngine(kubeClient, sanitizer.Namespace)

	// Ensure namespace is ready (wait if terminating, create if missing)
	if err := engine.EnsureNamespace(ctx); err != nil {
		return fmt.Errorf("namespace setup failed: %w", err)
	}

	// Pre-cleanup to remove any leftover workloads from previous runs
	if err := engine.CleanupWorkloads(ctx); err != nil {
		return fmt.Errorf("pre-cleanup failed: %w", err)
	}

	// Create workloads with timing
	created, err := engine.RunTimed(ctx, replayLog, replaySpeed, false)
	if err != nil {
		return err
	}
	fmt.Printf("Applied %d events, waiting for stabilization (Ctrl+C to skip)...\n", created)

	// Set up timeout context if configured
	waitCtx := ctx
	if replayTimeout > 0 {
		var waitCancel context.CancelFunc
		waitCtx, waitCancel = context.WithTimeout(ctx, replayTimeout)
		defer waitCancel()
	}

	// Detect if stdout is a TTY for proper status formatting
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	var lastStatus string

	// Wait for stable
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- engine.WaitForStable(waitCtx, 30*time.Second, func(s replay.Status) {
			status := s.String()
			if isTTY {
				fmt.Printf("\r  %s", status)
			} else if status != lastStatus {
				// Only print when status changes in non-TTY mode
				fmt.Printf("  %s\n", status)
				lastStatus = status
			}
		})
	}()

	select {
	case <-ctx.Done():
		// Already interrupted
	case err := <-waitDone:
		if isTTY {
			fmt.Println()
		}
		if err != nil {
			if err == context.DeadlineExceeded {
				fmt.Fprintf(os.Stderr, "Warning: stabilization timed out after %v\n", replayTimeout)
			} else if err != context.Canceled {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		} else {
			fmt.Println("Stabilized!")
		}
	}

	// Cleanup
	fmt.Println("Cleaning up...")
	_ = engine.CleanupWorkloads(context.Background())
	fmt.Println("Done")

	return nil
}

// eventSummary returns a summary string for a replay log.
func eventSummary(log *format.ReplayLog) string {
	var deployments, jobs, scaleEvents int
	for _, event := range log.Events {
		switch event.Type {
		case format.EventCreate:
			switch event.Kind {
			case format.KindDeployment:
				deployments++
			case format.KindJob:
				jobs++
			}
		case format.EventScale:
			scaleEvents++
		}
	}
	return fmt.Sprintf("%d deployments, %d jobs, %d scale events", deployments, jobs, scaleEvents)
}

func validateReplayLog(log *format.ReplayLog) error {
	for i, event := range log.Events {
		switch event.Type {
		case format.EventCreate:
			switch event.Kind {
			case format.KindDeployment:
				if event.Deployment == nil {
					return fmt.Errorf("event %d: deployment create event has nil deployment", i)
				}
			case format.KindJob:
				if event.Job == nil {
					return fmt.Errorf("event %d: job create event has nil job", i)
				}
			default:
				return fmt.Errorf("event %d: unknown workload kind %q", i, event.Kind)
			}
		case format.EventScale:
			if event.Replicas == nil {
				return fmt.Errorf("event %d: scale event has nil replicas", i)
			}
		default:
			return fmt.Errorf("event %d: unknown event type %q", i, event.Type)
		}
	}
	return nil
}

