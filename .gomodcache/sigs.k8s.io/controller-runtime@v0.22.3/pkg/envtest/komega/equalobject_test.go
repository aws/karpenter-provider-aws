package komega

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEqualObjectMatcher(t *testing.T) {
	cases := []struct {
		name     string
		original client.Object
		modified client.Object
		options  []EqualObjectOption
		want     bool
	}{
		{
			name: "succeed with equal objects",
			original: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			modified: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			want: true,
		},
		{
			name: "fail with non equal objects",
			original: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			modified: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somethingelse",
				},
			},
			want: false,
		},
		{
			name: "succeeds if ignored fields do not match",
			original: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"somelabel": "somevalue"},
					OwnerReferences: []metav1.OwnerReference{{
						Name: "controller",
					}},
				},
			},
			modified: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "somethingelse",
					Labels: map[string]string{"somelabel": "anothervalue"},
					OwnerReferences: []metav1.OwnerReference{{
						Name: "another",
					}},
				},
			},
			want: true,
			options: []EqualObjectOption{
				IgnorePaths{
					"ObjectMeta.Name",
					"ObjectMeta.CreationTimestamp",
					"ObjectMeta.Labels.somelabel",
					"ObjectMeta.OwnerReferences[0].Name",
					"Spec.Template.ObjectMeta",
				},
			},
		},
		{
			name: "succeeds if ignored fields in json notation do not match",
			original: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"somelabel": "somevalue"},
					OwnerReferences: []metav1.OwnerReference{{
						Name: "controller",
					}},
				},
			},
			modified: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "somethingelse",
					Labels: map[string]string{"somelabel": "anothervalue"},
					OwnerReferences: []metav1.OwnerReference{{
						Name: "another",
					}},
				},
			},
			want: true,
			options: []EqualObjectOption{
				IgnorePaths{
					"metadata.name",
					"metadata.creationTimestamp",
					"metadata.labels.somelabel",
					"metadata.ownerReferences[0].name",
					"spec.template.metadata",
				},
			},
		},
		{
			name: "succeeds if all allowed fields match, and some others do not",
			original: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			modified: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "special",
				},
			},
			want: true,
			options: []EqualObjectOption{
				MatchPaths{
					"ObjectMeta.Name",
				},
			},
		},
		{
			name: "works with unstructured.Unstructured",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "something",
						"namespace": "test",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "somethingelse",
						"namespace": "test",
					},
				},
			},
			want: true,
			options: []EqualObjectOption{
				IgnorePaths{
					"metadata.name",
				},
			},
		},

		// Test when objects are equal.
		{
			name: "Equal field (spec) both in original and in modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: true,
		},

		{
			name: "Equal nested field both in original and in modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"A": "A",
							},
						},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"A": "A",
							},
						},
					},
				},
			},
			want: true,
		},

		// Test when there is a difference between the objects.
		{
			name: "Unequal field both in original and in modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"foo": "bar-changed",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
		{
			name: "Unequal nested field both in original and modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"A": "A-Changed",
							},
						},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"A": "A",
							},
						},
					},
				},
			},
			want: false,
		},

		{
			name: "Value of type map with different values",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"map": map[string]string{
							"A": "A-changed",
							"B": "B",
							// C missing
						},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"map": map[string]string{
							"A": "A",
							// B missing
							"C": "C",
						},
					},
				},
			},
			want: false,
		},

		{
			name: "Value of type Array or Slice with same length but different values",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"slice": []string{
							"D",
							"C",
							"B",
						},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"slice": []string{
							"A",
							"B",
							"C",
						},
					},
				},
			},
			want: false,
		},

		// This tests specific behaviour in how Kubernetes marshals the zero value of metav1.Time{}.
		{
			name: "Creation timestamp set to empty value on both original and modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
					"metadata": map[string]interface{}{
						"selfLink":          "foo",
						"creationTimestamp": metav1.Time{},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
					"metadata": map[string]interface{}{
						"selfLink":          "foo",
						"creationTimestamp": metav1.Time{},
					},
				},
			},
			want: true,
		},

		// Cases to test diff when fields exist only in modified object.
		{
			name: "Field only in modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
		{
			name: "Nested field only in modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"A": "A",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Creation timestamp exists on modified but not on original",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
					"metadata": map[string]interface{}{
						"selfLink":          "foo",
						"creationTimestamp": "2021-11-03T11:05:17Z",
					},
				},
			},
			want: false,
		},

		// Test when fields exists only in the original object.
		{
			name: "Field only in original",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			want: false,
		},
		{
			name: "Nested field only in original",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"A": "A",
							},
						},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			want: false,
		},
		{
			name: "Creation timestamp exists on original but not on modified",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
					"metadata": map[string]interface{}{
						"selfLink":          "foo",
						"creationTimestamp": "2021-11-03T11:05:17Z",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
				},
			},

			want: false,
		},

		// Test metadata fields computed by the system or in status are compared.
		{
			name: "Unequal Metadata fields computed by the system or in status",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"selfLink":        "foo",
						"uid":             "foo",
						"resourceVersion": "foo",
						"generation":      "foo",
						"managedFields":   "foo",
					},
					"status": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
		{
			name: "Unequal labels and annotations",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
						},
						"annotations": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			want: false,
		},

		// Ignore fields MatchOption
		{
			name: "Unequal metadata fields ignored by IgnorePaths MatchOption",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":            "test",
						"selfLink":        "foo",
						"uid":             "foo",
						"resourceVersion": "foo",
						"generation":      "foo",
						"managedFields":   "foo",
					},
				},
			},
			options: []EqualObjectOption{IgnoreAutogeneratedMetadata},
			want:    true,
		},
		{
			name: "Unequal labels and annotations ignored by IgnorePaths MatchOption",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"labels": map[string]interface{}{
							"foo": "bar",
						},
						"annotations": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			options: []EqualObjectOption{IgnorePaths{"metadata.labels", "metadata.annotations"}},
			want:    true,
		},
		{
			name: "Ignore fields are not compared",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"controlPlaneEndpoint": map[string]interface{}{
							"host": "",
							"port": 0,
						},
					},
				},
			},
			options: []EqualObjectOption{IgnorePaths{"spec.controlPlaneEndpoint"}},
			want:    true,
		},
		{
			name: "Not-ignored fields are still compared",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{},
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"ignored":    "somevalue",
							"superflous": "shouldcausefailure",
						},
					},
				},
			},
			options: []EqualObjectOption{IgnorePaths{"metadata.annotations.ignored"}},
			want:    false,
		},

		// MatchPaths MatchOption
		{
			name: "Unequal metadata fields not compared by setting MatchPaths MatchOption",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
					},
					"metadata": map[string]interface{}{
						"selfLink": "foo",
						"uid":      "foo",
					},
				},
			},
			options: []EqualObjectOption{MatchPaths{"spec"}},
			want:    true,
		},

		// More tests
		{
			name: "No changes",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
						"B": "B",
						"C": "C", // C only in original
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
						"B": "B",
					},
				},
			},
			want: false,
		},
		{
			name: "Many changes",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
						// B missing
						"C": "C", // C only in original
					},
				},
			},
			modified: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"A": "A",
						"B": "B",
					},
				},
			},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewWithT(t)
			m := EqualObject(c.original, c.options...)
			success, _ := m.Match(c.modified)
			if !success {
				t.Log(m.FailureMessage(c.modified))
			}
			g.Expect(success).To(Equal(c.want))
		})
	}
}
