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

package common

import (
	"time"

	. "github.com/onsi/gomega" //nolint:stylecheck
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "sigs.k8s.io/karpenter/pkg/apis/autoscaling/v1alpha1"
)

func (env *Environment) EventuallyExpectCapacityBufferReady(buffer *autoscalingv1alpha1.CapacityBuffer) {
	Eventually(func(g Gomega) {
		cb := &autoscalingv1alpha1.CapacityBuffer{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(buffer), cb)).To(Succeed())
		cond := findBufferCondition(cb.Status.Conditions, autoscalingv1alpha1.ReadyForProvisioningCondition)
		g.Expect(cond).ToNot(BeNil())
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	}).WithTimeout(30 * time.Second).Should(Succeed())
}

func (env *Environment) EventuallyExpectCapacityBufferProvisioned(buffer *autoscalingv1alpha1.CapacityBuffer) {
	Eventually(func(g Gomega) {
		cb := &autoscalingv1alpha1.CapacityBuffer{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(buffer), cb)).To(Succeed())
		cond := findBufferCondition(cb.Status.Conditions, autoscalingv1alpha1.ProvisioningCondition)
		g.Expect(cond).ToNot(BeNil())
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	}).WithTimeout(5 * time.Minute).Should(Succeed())
}

func (env *Environment) EventuallyExpectCapacityBufferReplicas(buffer *autoscalingv1alpha1.CapacityBuffer, replicas int32) {
	Eventually(func(g Gomega) {
		cb := &autoscalingv1alpha1.CapacityBuffer{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(buffer), cb)).To(Succeed())
		cond := findBufferCondition(cb.Status.Conditions, autoscalingv1alpha1.ReadyForProvisioningCondition)
		g.Expect(cond).ToNot(BeNil())
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		g.Expect(cb.Status.Replicas).ToNot(BeNil())
		g.Expect(*cb.Status.Replicas).To(Equal(replicas))
	}).WithTimeout(30 * time.Second).Should(Succeed())
}

func (env *Environment) EventuallyExpectCapacityBufferNotReady(buffer *autoscalingv1alpha1.CapacityBuffer, reason string) {
	Eventually(func(g Gomega) {
		cb := &autoscalingv1alpha1.CapacityBuffer{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(buffer), cb)).To(Succeed())
		cond := findBufferCondition(cb.Status.Conditions, autoscalingv1alpha1.ReadyForProvisioningCondition)
		g.Expect(cond).ToNot(BeNil())
		g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(cond.Reason).To(Equal(reason))
	}).WithTimeout(30 * time.Second).Should(Succeed())
}

func findBufferCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
