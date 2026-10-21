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
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type EBSCSIDriver struct {
	Provisioner   string
	ZoneLabel     string
	NodeDaemonSet string
}

var DefaultEBSCSIDriver = EBSCSIDriver{
	Provisioner:   "ebs.csi.aws.com",
	ZoneLabel:     "topology.ebs.csi.aws.com/zone",
	NodeDaemonSet: "ebs-csi-node",
}

func (env *Environment) RequireEBSCSIDriver() {
	GinkgoHelper()
	err := env.Client.Get(env.Context, client.ObjectKey{Name: env.EBSCSIDriver.Provisioner}, &storagev1.CSIDriver{})
	if errors.IsNotFound(err) {
		Skip(fmt.Sprintf("[unsupported:ebs-csi-driver] CSIDriver %s is not installed", env.EBSCSIDriver.Provisioner))
	}
	Expect(err).ToNot(HaveOccurred())
}

func (env *Environment) RequireEBSCSINodeDaemonSet(need string) {
	GinkgoHelper()
	if env.EBSCSIDriver.NodeDaemonSet == "" {
		Skip(fmt.Sprintf("[unsupported:ebs-csi-driver-config] %s needs the EBS CSI node DaemonSet; %s runs without one", need, env.EBSCSIDriver.Provisioner))
	}
}
