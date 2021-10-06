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
	"strings"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricNamespace = metrics.KarpenterNamespace
	metricSubsystem = "capacity"

	metricLabelArch         = "arch"
	metricLabelInstanceType = "instancetype"
	metricLabelOs           = "os"
	metricLabelProvisioner  = metrics.ProvisionerLabel
	metricLabelZone         = "zone"

	nodeLabelArch         = v1.LabelArchStable
	nodeLabelInstanceType = v1.LabelInstanceTypeStable
	nodeLabelOs           = v1.LabelOSStable
	nodeLabelZone         = v1.LabelTopologyZone

	nodeConditionTypeReady = v1.NodeReady
)

type (
	nodeListConsumerFunc = func([]v1.Node) error
	consumeNodesWithFunc = func(client.MatchingLabels, nodeListConsumerFunc) error
)

var (
	nodeLabelProvisioner = v1alpha4.ProvisionerNameLabelKey

	knownValuesForNodeLabels = v1alpha4.WellKnownLabels

	nodeCountByProvisioner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "node_count",
			Help:      "Total node count by provisioner.",
		},
		[]string{
			metricLabelProvisioner,
		},
	)

	readyNodeCountByProvisionerZone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "ready_node_count",
			Help:      "Count of nodes that are ready by provisioner and zone.",
		},
		[]string{
			metricLabelProvisioner,
			metricLabelZone,
		},
	)

	readyNodeCountByArchProvisionerZone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "ready_node_arch_count",
			Help:      "Count of nodes that are ready by architecture, provisioner, and zone.",
		},
		[]string{
			metricLabelArch,
			metricLabelProvisioner,
			metricLabelZone,
		},
	)

	readyNodeCountByInstancetypeProvisionerZone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "ready_node_instancetype_count",
			Help:      "Count of nodes that are ready by instance type, provisioner, and zone.",
		},
		[]string{
			metricLabelInstanceType,
			metricLabelProvisioner,
			metricLabelZone,
		},
	)

	readyNodeCountByOsProvisionerZone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "ready_node_os_count",
			Help:      "Count of nodes that are ready by operating system, provisioner, and zone.",
		},
		[]string{
			metricLabelOs,
			metricLabelProvisioner,
			metricLabelZone,
		},
	)
)

func init() {
	crmetrics.Registry.MustRegister(nodeCountByProvisioner)
	crmetrics.Registry.MustRegister(readyNodeCountByProvisionerZone)
	crmetrics.Registry.MustRegister(readyNodeCountByArchProvisionerZone)
	crmetrics.Registry.MustRegister(readyNodeCountByInstancetypeProvisionerZone)
	crmetrics.Registry.MustRegister(readyNodeCountByOsProvisionerZone)
}

func publishNodeCountsForProvisioner(provisioner string, consumeNodesWith consumeNodesWithFunc) error {
	archValues := knownValuesForNodeLabels[nodeLabelArch]
	instanceTypeValues := knownValuesForNodeLabels[nodeLabelInstanceType]
	osValues := knownValuesForNodeLabels[nodeLabelOs]
	zoneValues := knownValuesForNodeLabels[nodeLabelZone]

	errors := make([]error, 0, len(archValues)*len(instanceTypeValues)*len(osValues)*len(zoneValues))

	// 1. Publish the count of all nodes associated with `provisioner`.
	nodeLabels := client.MatchingLabels{nodeLabelProvisioner: provisioner}
	errors = append(errors, consumeNodesWith(nodeLabels, func(nodes []v1.Node) error {
		return publishCount(nodeCountByProvisioner, metricLabelsFromNodeLabels(nodeLabels), len(nodes))
	}))

	for _, zone := range zoneValues {
		// 2. Publish the count of all nodes associated with `provisioner`, in `zone`, and reported as "ready".
		nodeLabels = client.MatchingLabels{
			nodeLabelProvisioner: provisioner,
			nodeLabelZone:        zone,
		}
		errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
			return publishCount(readyNodeCountByProvisionerZone, metricLabelsFromNodeLabels(nodeLabels), len(readyNodes))
		})))

		for _, arch := range archValues {
			// 3. Publish the count of all nodes with `arch`, associated with `provisioner`, in `zone`, and reported as "ready".
			nodeLabels := client.MatchingLabels{
				nodeLabelArch:        arch,
				nodeLabelProvisioner: provisioner,
				nodeLabelZone:        zone,
			}
			errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
				return publishCount(readyNodeCountByArchProvisionerZone, metricLabelsFromNodeLabels(nodeLabels), len(readyNodes))
			})))
		}

		for _, instanceType := range instanceTypeValues {
			// 4. Publish the count of all nodes with `instanceType`, associated with `provisioner`, in `zone`, and reported as "ready"
			nodeLabels := client.MatchingLabels{
				nodeLabelInstanceType: instanceType,
				nodeLabelProvisioner:  provisioner,
				nodeLabelZone:         zone,
			}
			errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
				return publishCount(readyNodeCountByInstancetypeProvisionerZone, metricLabelsFromNodeLabels(nodeLabels), len(readyNodes))
			})))
		}

		for _, os := range osValues {
			// 5. Publish the count of all nodes with `os`, associated with `provisioner`, in `zone`, and reported as "ready".
			nodeLabels := client.MatchingLabels{
				nodeLabelOs:          os,
				nodeLabelProvisioner: provisioner,
				nodeLabelZone:        zone,
			}
			errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
				return publishCount(readyNodeCountByOsProvisionerZone, metricLabelsFromNodeLabels(nodeLabels), len(readyNodes))
			})))
		}
	}

	// Combine will filter out `nil` values; if no errors remain then it will return `nil`.
	return multierr.Combine(errors...)
}

// filterReadyNodes returns a new function that will filter "ready" nodes to pass on
// to `consume`, and returns the result.
func filterReadyNodes(consume nodeListConsumerFunc) nodeListConsumerFunc {
	return func(nodes []v1.Node) error {
		readyNodes := make([]v1.Node, 0, len(nodes))
		for _, node := range nodes {
			for _, condition := range node.Status.Conditions {
				if condition.Type == nodeConditionTypeReady && strings.ToLower(string(condition.Status)) == "true" {
					readyNodes = append(readyNodes, node)
				}
			}
		}
		return consume(readyNodes)
	}
}

func metricLabelsFromNodeLabels(nodeLabels client.MatchingLabels) (metricLabels prometheus.Labels) {
	metricLabels = prometheus.Labels{}
	// Exclude node label values that not present or are empty strings.
	if arch := nodeLabels[nodeLabelArch]; arch != "" {
		metricLabels[metricLabelArch] = arch
	}
	if instanceType := nodeLabels[nodeLabelInstanceType]; instanceType != "" {
		metricLabels[metricLabelInstanceType] = instanceType
	}
	if os := nodeLabels[nodeLabelOs]; os != "" {
		metricLabels[metricLabelOs] = os
	}
	if provisioner := nodeLabels[nodeLabelProvisioner]; provisioner != "" {
		metricLabels[metricLabelProvisioner] = provisioner
	}
	if zone := nodeLabels[nodeLabelZone]; zone != "" {
		metricLabels[metricLabelZone] = zone
	}
	return
}

func publishCount(gaugeVec *prometheus.GaugeVec, labels prometheus.Labels, count int) (err error) {
	var gauge prometheus.Gauge
	gauge, err = gaugeVec.GetMetricWith(labels)
	if err != nil {
		return
	}
	gauge.Set(float64(count))
	return
}
