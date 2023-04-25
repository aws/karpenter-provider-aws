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

package test

import (
	"encoding/base64"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/gomega" // nolint:revive,stylecheck
)

func (env Environment) ExpectLaunchTemplatesCreatedWithUserDataContaining(substrings ...string) {
	Expect(env.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	env.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		Expect(err).To(BeNil())
		for _, substring := range substrings {
			Expect(string(userData)).To(ContainSubstring(substring))
		}
	})
}

func (env Environment) ExpectLaunchTemplatesCreatedWithUserData(expected string) {
	Expect(env.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	env.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		Expect(err).To(BeNil())
		// Newlines are always added for missing TOML fields, so strip them out before comparisons.
		actualUserData := strings.Replace(string(userData), "\n", "", -1)
		expectedUserData := strings.Replace(expected, "\n", "", -1)
		Expect(expectedUserData).To(Equal(actualUserData))
	})
}
