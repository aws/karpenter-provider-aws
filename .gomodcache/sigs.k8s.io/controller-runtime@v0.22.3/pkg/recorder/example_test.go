/*
Copyright 2018 The Kubernetes Authors.

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

package recorder_test

import (
	corev1 "k8s.io/api/core/v1"

	_ "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
)

var (
	recorderProvider recorder.Provider
	somePod          *corev1.Pod // the object you're reconciling, for example
)

func Example_event() {
	// recorderProvider is a recorder.Provider
	recorder := recorderProvider.GetEventRecorderFor("my-controller")

	// emit an event with a fixed message
	recorder.Event(somePod, corev1.EventTypeWarning,
		"WrongTrousers", "It's the wrong trousers, Gromit!")
}

func Example_eventf() {
	// recorderProvider is a recorder.Provider
	recorder := recorderProvider.GetEventRecorderFor("my-controller")

	// emit an event with a variable message
	mildCheese := "Wensleydale"
	recorder.Eventf(somePod, corev1.EventTypeNormal,
		"DislikesCheese", "Not even %s?", mildCheese)
}
