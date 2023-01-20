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
	"log"
	"os"
)

const (
	envVarQueueURL          = "QUEUE_URL"
	envVarQueueAWSRegion    = "QUEUE_AWS_REGION"
	envVarAWSRegion         = "AWS_REGION"
	envVarTektonClusterName = "CLUSTER_NAME"
	envGithubAccount        = "GITHUB_ACCOUNT"
)

type config struct {
	queueURL          string
	queueRegion       string
	region            string
	tektonClusterName string
	githubAccount     string
}

// Start configures clients and starts listening for messages containing release notifications
func Start() {
	config := getConfig()
	log.Printf("config values: %#v", config)

	getSession(config)
	pollMessages(config)
}

func getConfig() *config {
	return &config{
		queueURL:          os.Getenv(envVarQueueURL),
		queueRegion:       os.Getenv(envVarQueueAWSRegion),
		region:            os.Getenv(envVarAWSRegion),
		tektonClusterName: os.Getenv(envVarTektonClusterName),
		githubAccount:     os.Getenv(envGithubAccount),
	}
}
