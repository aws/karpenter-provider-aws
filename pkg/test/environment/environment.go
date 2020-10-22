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

package environment

// Environment encapsulates a testing environment, such as a local API server or kubernetes test cluster.
type Environment interface {
	// Start must be called before the environment is used to setup resources.
	Start() error
	// Stop must be called after the environment is used to clean up resources.
	Stop() error
	// Namespace instantiates a new kubernetes namespace for testing. The namespace
	// will be automatically created when instantiated and deleted during env.Stop().
	NewNamespace() (*Namespace, error)
}
