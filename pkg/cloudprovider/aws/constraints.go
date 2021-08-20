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

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"knative.dev/pkg/apis"
)

const (
	CapacityTypeSpot             = "spot"
	CapacityTypeOnDemand         = "on-demand"
	DefaultLaunchTemplateVersion = "$Default"
)

var (
	AWSLabelPrefix           = "node.k8s.aws/"
	CapacityTypeLabel        = AWSLabelPrefix + "capacity-type"
	LaunchTemplateNameLabel  = AWSLabelPrefix + "launch-template-name"
	SubnetNameLabel          = AWSLabelPrefix + "subnet-name"
	SubnetTagKeyLabel        = AWSLabelPrefix + "subnet-tag-key"
	SecurityGroupNameLabel   = AWSLabelPrefix + "security-group-name"
	SecurityGroupTagKeyLabel = AWSLabelPrefix + "security-group-tag-key"
	AllowedLabels            = []string{
		CapacityTypeLabel,
		LaunchTemplateNameLabel,
		SubnetNameLabel,
		SubnetTagKeyLabel,
		SecurityGroupNameLabel,
		SecurityGroupTagKeyLabel,
	}
	AWSToKubeArchitectures = map[string]string{
		"x86_64":                   v1alpha3.ArchitectureAmd64,
		v1alpha3.ArchitectureArm64: v1alpha3.ArchitectureArm64,
	}
	KubeToAWSArchitectures = functional.InvertStringMap(AWSToKubeArchitectures)
)

// Constraints are AWS specific constraints
type Constraints struct {
	v1alpha3.Constraints
}

func (c *Constraints) GetCapacityType() string {
	capacityType, ok := c.Labels[CapacityTypeLabel]
	if !ok {
		capacityType = CapacityTypeOnDemand
	}
	return capacityType
}

type LaunchTemplate struct {
	Name    string
	Version string
}

func (c *Constraints) GetLaunchTemplate() *LaunchTemplate {
	name, ok := c.Labels[LaunchTemplateNameLabel]
	if !ok {
		return nil
	}
	return &LaunchTemplate{
		Name:    name,
		Version: DefaultLaunchTemplateVersion,
	}
}

func (c *Constraints) GetSubnetName() *string {
	name, ok := c.Labels[SubnetNameLabel]
	if !ok {
		return nil
	}
	return aws.String(name)
}

func (c *Constraints) GetSubnetTagKey() *string {
	tag, ok := c.Labels[SubnetTagKeyLabel]
	if !ok {
		return nil
	}
	return aws.String(tag)
}

func (c *Constraints) GetSecurityGroupName() *string {
	name, ok := c.Labels[SecurityGroupNameLabel]
	if !ok {
		return nil
	}
	return aws.String(name)
}

func (c *Constraints) GetSecurityGroupTagKey() *string {
	tag, ok := c.Labels[SecurityGroupTagKeyLabel]
	if !ok {
		return nil
	}
	return aws.String(tag)
}

func (c *Constraints) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		c.validateAllowedLabels(ctx),
		c.validateCapacityType(ctx),
		c.validateLaunchTemplate(ctx),
		c.validateSubnets(ctx),
	)
}

func (c *Constraints) validateAllowedLabels(ctx context.Context) (errs *apis.FieldError) {
	for key := range c.Labels {
		if strings.HasPrefix(key, AWSLabelPrefix) && !functional.ContainsString(AllowedLabels, key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "spec.labels"))
		}
	}
	return errs
}

func (c *Constraints) validateCapacityType(ctx context.Context) (errs *apis.FieldError) {
	capacityType, ok := c.Labels[CapacityTypeLabel]
	if !ok {
		return nil
	}
	capacityTypes := []string{CapacityTypeSpot, CapacityTypeOnDemand}
	if !functional.ContainsString(capacityTypes, capacityType) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", capacityType, capacityTypes), fmt.Sprintf("spec.labels[%s]", CapacityTypeLabel)))
	}
	return errs
}

func (c *Constraints) validateLaunchTemplate(ctx context.Context) (errs *apis.FieldError) {
	// nothing to validate at the moment
	return errs
}

func (c *Constraints) validateSubnets(ctx context.Context) (errs *apis.FieldError) {
	if c.GetSubnetName() != nil && c.GetSubnetTagKey() != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(fmt.Sprintf("spec.labels[%s]", SubnetNameLabel), fmt.Sprintf("spec.labels[%s]", SubnetTagKeyLabel)))
	}
	return errs
}
