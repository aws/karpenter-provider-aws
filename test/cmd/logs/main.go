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
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"
)

const (
	region       = "us-east-2"
	logGroupName = "KITInfrastructure"
	s3BucketName = "karpenter-ci-beta-us-east-2-test-logs"
)

type KTLogs struct {
	cwl      *cloudwatchlogs.Client
	s3client *s3.Client
}

func main() {
	ctx := context.Background()

	testID := os.Args[1]

	// Load the Shared AWS Configuration (~/.aws/config)
	cfg := lo.Must(config.LoadDefaultConfig(ctx))
	cfg.Region = region

	ktLogs := KTLogs{
		cwl:      cloudwatchlogs.NewFromConfig(cfg),
		s3client: s3.NewFromConfig(cfg),
	}

	if err := ktLogs.readS3Logs(ctx, testID, s3BucketName); err == nil {
		return
	}
	if err := ktLogs.exportLogsToS3(ctx, testID, s3BucketName); err != nil {
		logging.FromContext(ctx).Fatalf(err.Error())
	}

	lo.Must0(ktLogs.readS3Logs(ctx, testID, s3BucketName))
}

func (k KTLogs) exportLogsToS3(ctx context.Context, testID string, s3BucketName string) error {
	task, err := k.cwl.CreateExportTask(ctx, &cloudwatchlogs.CreateExportTaskInput{
		Destination:         aws.String(s3BucketName),
		LogGroupName:        aws.String(logGroupName),
		LogStreamNamePrefix: aws.String(fmt.Sprintf("fluentbit-kube.var.log.containers.%s-", testID)),
		DestinationPrefix:   aws.String(testID),
		From:                aws.Int64(time.Now().Add(-1 * time.Hour * 168).UnixMilli()),
		To:                  aws.Int64(time.Now().UnixMilli()),
	})
	if err != nil {
		return fmt.Errorf("unable to create log export task from CloudWatch Logs to S3, %w", err)
	}
	taskStatus := types.ExportTaskStatus{
		Code: types.ExportTaskStatusCodePending,
	}
	for taskStatus.Code != types.ExportTaskStatusCodePending && taskStatus.Code != types.ExportTaskStatusCodeRunning {
		time.Sleep(time.Second * 5)
		status, err := k.cwl.DescribeExportTasks(ctx, &cloudwatchlogs.DescribeExportTasksInput{TaskId: task.TaskId})
		if err != nil {
			log.Fatalf("Unable to get export task status from CloudWatch Logs to S3: %v", err)
		}
		if len(status.ExportTasks) > 0 {
			taskStatus = *status.ExportTasks[0].Status
			log.Printf("Export status %s\n", taskStatus.Code)
		}
	}
	return nil
}

func (k KTLogs) readS3Logs(ctx context.Context, testID string, s3BucketName string) error {
	objs, err := k.s3client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s3BucketName),
		Prefix: aws.String(testID),
	})
	if err != nil {
		return fmt.Errorf("unable to list logs in s3: %w", err)
	}
	if len(objs.Contents) <= 1 {
		return fmt.Errorf("no logs available in s3")
	}
	for _, obj := range objs.Contents {
		if strings.Contains(*obj.Key, "aws-logs-write-test") {
			continue
		}
		out, err := k.s3client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s3BucketName),
			Key:    obj.Key,
		})
		if err != nil {
			log.Printf("Unable to get obj %s: %v", *obj.Key, err)
			continue
		}
		gr, err := gzip.NewReader(out.Body)
		if err != nil {
			log.Printf("Unable to read compressed body of %s: %v", *obj.Key, err)
			continue
		}
		defer gr.Close()
		objBytes, err := io.ReadAll(gr)
		if err != nil {
			log.Printf("Unable to decompress body of %s: %v", *obj.Key, err)
			continue
		}
		fmt.Println(string(objBytes))
	}
	return nil
}
