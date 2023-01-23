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

package listener

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const (
	maxNumberOfMessages               = 1
	delayBetweenMessageReads          = time.Minute * 3
	maxNotificationMessageParamLength = 40 // Length of a git SHA
	visibilityTimeOutS                = 60
	defaultKnownLastStableRelease     = "v0.22.1"

	releaseTypeStable   = "stable"
	releaseTypeSnapshot = "snapshot"
	releaseTypePeriodic = "periodic"
)

type notificationMessage struct {
	ReleaseType          string `json:"releaseType"`
	ReleaseIdentifier    string `json:"releaseIdentifier"`
	PrNumber             string `json:"prNumber"`
	GithubAccount        string `json:"githubAccount"`
	LastStableReleaseTag string `json:"lastStableReleaseTag"`
}

var (
	sqsSvc            *sqs.SQS
	awsCLICommandPath string
	validReleaseTypes = map[string]struct{}{
		releaseTypeStable:   {},
		releaseTypeSnapshot: {},
		releaseTypePeriodic: {},
	}
	lastKnownLastStableRelease string
)

func processMessage(queueMessage *sqs.Message, config *config) {
	notificationMessage, err := newNotificationMessage(queueMessage)
	if err != nil {
		log.Fatalf("failed parsing message. %#v, %s", notificationMessage, err)
	}
	if notificationMessage.GithubAccount != config.githubAccount { // Ignore fork messages
		log.Printf("github account %s does not match expected %s", notificationMessage.GithubAccount, config.githubAccount)
		return
	}
	log.Printf("running tests for notification message %#v", notificationMessage)

	for pipeline, filters := range pipelinesAndFilters {
		if len(filters) == 0 {
			runTektonCommand(notificationMessage, pipeline, "")
			continue
		}

		for _, filter := range filters {
			runTektonCommand(notificationMessage, pipeline, filter)
		}
	}

	if err := deleteMessage(config, queueMessage); err != nil {
		log.Fatalf("failed deleting message. %s", err)
	}
}

func runTektonCommand(notificationMessage *notificationMessage, pipeline string, filter string) {
	tknArgs := tknArgs(notificationMessage, pipeline, filter)
	if err := runTests(notificationMessage, tknArgs...); err != nil {
		log.Printf("failed running pipeline %s tests on message. %s", pipeline, err)
	}
}

func newNotificationMessage(msg *sqs.Message) (*notificationMessage, error) {
	var queueMessage *notificationMessage
	if err := json.Unmarshal([]byte(*msg.Body), &queueMessage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json. %w", err)
	}

	if err := queueMessage.validate(); err != nil {
		return nil, fmt.Errorf("invalid message. %w", err)
	}

	if queueMessage.PrNumber == "" {
		queueMessage.PrNumber = noPrNumber
	}
	return queueMessage, nil
}

func (n *notificationMessage) lastStableReleaseTagOrDefault() string {
	if n.LastStableReleaseTag != "" {
		lastKnownLastStableRelease = n.LastStableReleaseTag
		return n.LastStableReleaseTag
	}
	if lastKnownLastStableRelease != "" {
		return lastKnownLastStableRelease
	}
	return defaultKnownLastStableRelease
}

func (n *notificationMessage) validate() error {
	if len(n.ReleaseIdentifier) > maxNotificationMessageParamLength || len(n.ReleaseIdentifier) == 0 {
		return errors.New("releaseIdentifier too long or empty")
	}
	if len(n.PrNumber) > maxNotificationMessageParamLength {
		return errors.New("prNumber too long")
	}
	if _, ok := validReleaseTypes[n.ReleaseType]; !ok {
		return fmt.Errorf("unknown release type %s", n.ReleaseType)
	}
	return nil
}

func getSession(config *config) {
	sqsSvc = sqs.New(session.Must(session.NewSessionWithOptions(
		session.Options{Config: aws.Config{Region: aws.String(config.queueRegion)}},
	)))

	var err error
	if awsCLICommandPath, err = cmdPath("aws"); err != nil {
		log.Fatalf("failed to find path for aws. %s", err)
	}
	if tektonCLICommandPath, err = cmdPath("tkn"); err != nil {
		log.Fatalf("failed to find path for tkn. %s", err)
	}
	if err := authenticateEKS(config); err != nil {
		log.Fatalf("failed to authenticate against eks. %s", err)
	}
}

func pollMessages(config *config) {
	log.Printf("polling messages from %s", config.queueURL)
	for {
		output, err := sqsSvc.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(config.queueURL),
			MaxNumberOfMessages: aws.Int64(maxNumberOfMessages),
			VisibilityTimeout:   aws.Int64(visibilityTimeOutS),
		})

		if err != nil {
			log.Fatalf("failed to fetch message %s", err)
		}

		for _, queueMessage := range output.Messages {
			processMessage(queueMessage, config)
			time.Sleep(delayBetweenMessageReads)
		}
	}
}

func deleteMessage(config *config, msg *sqs.Message) error {
	log.Printf("deleting msg %#v", msg)
	_, err := sqsSvc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String(config.queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
	return err
}

func cmdPath(cmd string) (string, error) {
	cmdPath, err := exec.LookPath(cmd)
	if err != nil {
		return "", fmt.Errorf("failed finding the path to %s. %w", cmd, err)
	}
	return cmdPath, nil
}
