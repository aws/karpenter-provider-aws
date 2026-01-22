# Kubereplay Refactor Summary

## Problem
Pod-level replay conflated workload intent with node management artifacts. Pods recreated after node eviction appeared as new events, making A/B comparisons between Karpenter versions invalid.

## Solution
Track **Deployments and Jobs** instead of pods. These represent workload intent, letting controllers manage pod lifecycle.

## Changes Made

### Data Model (`pkg/format/log.go`)
- `WorkloadEvent` with types: `create`, `scale`
- Kinds: `Deployment`, `Job`
- Scale events track replicas changes

### Parser (`pkg/parser/audit.go`)
- Filters for `deployments` and `jobs` (not pods)
- Detects scale changes via `update`/`patch` and `deployments/scale` subresource
- **Pre-existing handling**: First `update`/`patch` for unknown deployment emits as `create`
- Tracks `deploymentEmitted` map to distinguish "seen replicas" from "emitted create"

### CloudWatch (`pkg/cloudwatch/client.go`)
- Filter: `{ ($.objectRef.resource = "deployments" || $.objectRef.resource = "jobs") && ($.verb = "create" || $.verb = "update" || $.verb = "patch") }`

### Sanitizer (`pkg/sanitizer/sanitizer.go`)
- New package (replaced `deidentifier`)
- `SanitizeDeployment()` and `SanitizeJob()`
- Preserves scheduling constraints, strips identifying info

### Replay Engine (`pkg/replay/engine.go`)
- Creates Deployments/Jobs instead of pods
- Handles scale events by patching replicas
- `deploymentNames` map tracks original key → replayed name for scale events
- `CleanupWorkloads()` replaces `CleanupPods()`

### Commands
- `capture.go` - Uses new parser/sanitizer
- `replay.go` - Uses new engine
- `demo.go` - Generates deployments with scale events

## Pre-existing Deployment Flow
```
Audit log timeline:
  [before window] deployment-x created
  [in window] scale event for deployment-x (replicas: 5)
  [in window] update event for deployment-x (full spec, replicas: 5)
  [in window] scale event for deployment-x (replicas: 10)

Parser output:
  1. Scale event seen first → track replicas=5, don't emit (no spec)
  2. Update event seen → emit CREATE (first full spec), mark emitted
  3. Scale event → emit SCALE (replicas changed 5→10)
```

## Testing
```bash
# Build
cd tools/kubereplay && go build -o kubereplay ./cmd

# Generate demo data
./kubereplay demo -o demo.json

# Dry-run replay
./kubereplay replay -f demo.json --dry-run --speed 3600
```
