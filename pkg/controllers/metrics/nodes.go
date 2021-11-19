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

package metrics

import (
	"strings"

	"github.com/awslabs/karpenter/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type (
	nodeListConsumerFunc = func([]v1.Node) error
	consumeNodesWithFunc = func(client.MatchingLabels, nodeListConsumerFunc) error
)

var (
	nodeCountByProvisioner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metricSubsystemCapacity,
			Name:      "node_count",
			Help:      "Total node count by provisioner.",
		},
		[]string{
			metricLabelProvisioner,
		},
	)

	readyNodeCountByProvisionerZone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metricSubsystemCapacity,
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
			Namespace: metrics.Namespace,
			Subsystem: metricSubsystemCapacity,
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
			Namespace: metrics.Namespace,
			Subsystem: metricSubsystemCapacity,
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
			Namespace: metrics.Namespace,
			Subsystem: metricSubsystemCapacity,
			Name:      "ready_node_os_count",
			Help:      "Count of nodes that are ready by provisioner, and zone.",
		},
		[]string{
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

func publishNodeCounts(provisioner string, knownValuesForNodeLabels map[string]sets.String, consumeNodesWith consumeNodesWithFunc) error {
	archValues := knownValuesForNodeLabels[nodeLabelArch]
	instanceTypeValues := knownValuesForNodeLabels[nodeLabelInstanceType]
	zoneValues := knownValuesForNodeLabels[nodeLabelZone]

	errors := make([]error, 0, len(archValues)*len(instanceTypeValues)*len(zoneValues))

	nodeLabels := client.MatchingLabels{nodeLabelProvisioner: provisioner}
	errors = append(errors, consumeNodesWith(nodeLabels, func(nodes []v1.Node) error {
		return publishCount(nodeCountByProvisioner, metricLabelsFrom(nodeLabels), len(nodes))
	}))

	for zone := range zoneValues {
		nodeLabels = client.MatchingLabels{
			nodeLabelProvisioner: provisioner,
			nodeLabelZone:        zone,
		}
		errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
			return publishCount(readyNodeCountByProvisionerZone, metricLabelsFrom(nodeLabels), len(readyNodes))
		})))

		for arch := range archValues {
			nodeLabels := client.MatchingLabels{
				nodeLabelArch:        arch,
				nodeLabelProvisioner: provisioner,
				nodeLabelZone:        zone,
			}
			errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
				return publishCount(readyNodeCountByArchProvisionerZone, metricLabelsFrom(nodeLabels), len(readyNodes))
			})))
		}

		for instanceType := range instanceTypeValues {
			nodeLabels := client.MatchingLabels{
				nodeLabelInstanceType: instanceType,
				nodeLabelProvisioner:  provisioner,
				nodeLabelZone:         zone,
			}
			errors = append(errors, consumeNodesWith(nodeLabels, filterReadyNodes(func(readyNodes []v1.Node) error {
				return publishCount(readyNodeCountByInstancetypeProvisionerZone, metricLabelsFrom(nodeLabels), len(readyNodes))
			})))
		}
	}

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

func metricLabelsFrom(nodeLabels map[string]string) prometheus.Labels {
	metricLabels := prometheus.Labels{}
	// Exclude node label values that not present or are empty strings.
	if arch := nodeLabels[nodeLabelArch]; arch != "" {
		metricLabels[metricLabelArch] = arch
	}
	if instanceType := nodeLabels[nodeLabelInstanceType]; instanceType != "" {
		metricLabels[metricLabelInstanceType] = instanceType
	}
	if provisioner := nodeLabels[nodeLabelProvisioner]; provisioner != "" {
		metricLabels[metricLabelProvisioner] = provisioner
	}
	if zone := nodeLabels[nodeLabelZone]; zone != "" {
		metricLabels[metricLabelZone] = zone
	}
	return metricLabels
}
