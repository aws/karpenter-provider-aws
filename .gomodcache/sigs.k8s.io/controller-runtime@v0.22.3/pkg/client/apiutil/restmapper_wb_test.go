/*
Copyright 2024 The Kubernetes Authors.

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

package apiutil

import (
	"testing"

	gmg "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
)

func TestLazyRestMapper_fetchGroupVersionResourcesLocked_CacheInvalidation(t *testing.T) {
	tests := []struct {
		name                                   string
		groupName                              string
		versions                               []string
		cachedAPIGroups, expectedAPIGroups     map[string]*metav1.APIGroup
		cachedKnownGroups, expectedKnownGroups map[string]*restmapper.APIGroupResources
	}{
		{
			name:      "Not found version for cached groupVersion in apiGroups and knownGroups",
			groupName: "group1",
			versions:  []string{"v1", "v2"},
			cachedAPIGroups: map[string]*metav1.APIGroup{
				"group1": {
					Name: "group1",
					Versions: []metav1.GroupVersionForDiscovery{
						{
							Version: "v1",
						},
					},
				},
			},
			cachedKnownGroups: map[string]*restmapper.APIGroupResources{
				"group1": {
					VersionedResources: map[string][]metav1.APIResource{
						"v1": {
							{
								Name: "resource1",
							},
						},
					},
				},
			},
			expectedAPIGroups:   map[string]*metav1.APIGroup{},
			expectedKnownGroups: map[string]*restmapper.APIGroupResources{},
		},
		{
			name:      "Not found version for cached groupVersion only in apiGroups",
			groupName: "group1",
			versions:  []string{"v1", "v2"},
			cachedAPIGroups: map[string]*metav1.APIGroup{
				"group1": {
					Name: "group1",
					Versions: []metav1.GroupVersionForDiscovery{
						{
							Version: "v1",
						},
					},
				},
			},
			cachedKnownGroups: map[string]*restmapper.APIGroupResources{
				"group1": {
					VersionedResources: map[string][]metav1.APIResource{
						"v3": {
							{
								Name: "resource1",
							},
						},
					},
				},
			},
			expectedAPIGroups: map[string]*metav1.APIGroup{},
			expectedKnownGroups: map[string]*restmapper.APIGroupResources{
				"group1": {
					VersionedResources: map[string][]metav1.APIResource{
						"v3": {
							{
								Name: "resource1",
							},
						},
					},
				},
			},
		},
		{
			name:      "Not found version for cached groupVersion only in knownGroups",
			groupName: "group1",
			versions:  []string{"v1", "v2"},
			cachedAPIGroups: map[string]*metav1.APIGroup{
				"group1": {
					Name: "group1",
					Versions: []metav1.GroupVersionForDiscovery{
						{
							Version: "v3",
						},
					},
				},
			},
			cachedKnownGroups: map[string]*restmapper.APIGroupResources{
				"group1": {
					VersionedResources: map[string][]metav1.APIResource{
						"v2": {
							{
								Name: "resource1",
							},
						},
					},
				},
			},
			expectedAPIGroups: map[string]*metav1.APIGroup{
				"group1": {
					Name: "group1",
					Versions: []metav1.GroupVersionForDiscovery{
						{
							Version: "v3",
						},
					},
				},
			},
			expectedKnownGroups: map[string]*restmapper.APIGroupResources{},
		},
		{
			name:      "Not found version for non cached groupVersion",
			groupName: "group1",
			versions:  []string{"v1", "v2"},
			cachedAPIGroups: map[string]*metav1.APIGroup{
				"group1": {
					Name: "group1",
					Versions: []metav1.GroupVersionForDiscovery{
						{
							Version: "v3",
						},
					},
				},
			},
			cachedKnownGroups: map[string]*restmapper.APIGroupResources{
				"group1": {
					VersionedResources: map[string][]metav1.APIResource{
						"v3": {
							{
								Name: "resource1",
							},
						},
					},
				},
			},
			expectedAPIGroups: map[string]*metav1.APIGroup{
				"group1": {
					Name: "group1",
					Versions: []metav1.GroupVersionForDiscovery{
						{
							Version: "v3",
						},
					},
				},
			},
			expectedKnownGroups: map[string]*restmapper.APIGroupResources{
				"group1": {
					VersionedResources: map[string][]metav1.APIResource{
						"v3": {
							{
								Name: "resource1",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gmg.NewWithT(t)
			m := &mapper{
				mapper:      restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{}),
				client:      &fakeAggregatedDiscoveryClient{DiscoveryInterface: fake.NewSimpleClientset().Discovery()},
				apiGroups:   tt.cachedAPIGroups,
				knownGroups: tt.cachedKnownGroups,
			}
			_, err := m.fetchGroupVersionResourcesLocked(tt.groupName, tt.versions...)
			g.Expect(err).NotTo(gmg.HaveOccurred())
			g.Expect(m.apiGroups).To(gmg.BeComparableTo(tt.expectedAPIGroups))
			g.Expect(m.knownGroups).To(gmg.BeComparableTo(tt.expectedKnownGroups))
		})
	}
}

type fakeAggregatedDiscoveryClient struct {
	discovery.DiscoveryInterface
}

func (f *fakeAggregatedDiscoveryClient) GroupsAndMaybeResources() (*metav1.APIGroupList, map[schema.GroupVersion]*metav1.APIResourceList, map[schema.GroupVersion]error, error) {
	groupList, err := f.DiscoveryInterface.ServerGroups()
	return groupList, nil, nil, err
}
