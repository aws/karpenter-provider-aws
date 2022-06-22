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

package config

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

const (
	paramBatchMaxDuration  = "batchMaxDuration"
	paramBatchIdleDuration = "batchIdleDuration"

	configMapName = "karpenter-global-settings"
)

// these values need to be synced with our templates/configmap.yaml
var defaultConfigMapData = map[string]string{
	paramBatchMaxDuration:  "10s",
	paramBatchIdleDuration: "1s",
}

type ChangeHandler func(c Config)

type Config interface {
	// OnChange is used to register a handler to be called when the configuration has been changed
	OnChange(handler ChangeHandler)

	// BatchMaxDuration returns the maximum batch duration
	BatchMaxDuration() time.Duration
	// BatchIdleDuration returns the maximum idle period used to extend a batch duration up to BatchMaxDuration
	BatchIdleDuration() time.Duration
}
type config struct {
	ctx context.Context

	dataMu            sync.RWMutex
	batchMaxDuration  time.Duration
	batchIdleDuration time.Duration

	// hash of the config map so we only notify watches if it has changed
	configHash uint64

	watcherMu sync.Mutex
	watchers  []ChangeHandler
}

func (c *config) BatchMaxDuration() time.Duration {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return c.batchMaxDuration
}

func (c *config) BatchIdleDuration() time.Duration {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return c.batchIdleDuration
}

func New(ctx context.Context, kubeClient *kubernetes.Clientset, iw *informer.InformedWatcher) (Config, error) {
	if iw.Namespace != system.Namespace() {
		return nil, fmt.Errorf("watcher configured for wrong namespace, expected %s found %s", system.Namespace(), iw.Namespace)
	}

	cfg := &config{ctx: ctx}
	logging.FromContext(ctx).Infof("loading config from %s/%s", system.Namespace(), configMapName)

	cm, err := kubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logging.FromContext(ctx).Errorf("config %s/%s not found, defaulting all values", iw.Namespace, configMapName)
		} else {
			return nil, err
		}
	}

	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	for k, v := range defaultConfigMapData {
		if _, found := cm.Data[k]; !found {
			cm.Data[k] = v
			logging.FromContext(ctx).Infof("applying default config value %s=%s", k, v)
		}
	}

	iw.Watch(configMapName, cfg.configMapChanged)
	return cfg, nil
}

func (c *config) OnChange(handler ChangeHandler) {
	c.watcherMu.Lock()
	defer c.watcherMu.Unlock()
	c.watchers = append(c.watchers, handler)
}

// hashCM hashes a
func hashCM(cm *v1.ConfigMap) uint64 {
	var keys []string
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	hasher := fnv.New64()
	for _, k := range keys {
		fmt.Fprint(hasher, k)
		fmt.Fprint(hasher, cm.Data[k])
	}

	keys = nil
	for k := range cm.BinaryData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprint(hasher, k)
		hasher.Write(cm.BinaryData[k])
	}
	return hasher.Sum64()
}

func (c *config) configMapChanged(configMap *v1.ConfigMap) {
	hash := hashCM(configMap)
	if hash == c.configHash {
		return
	}
	if c.configHash != 0 {
		logging.FromContext(c.ctx).Infof("configuration change detected")
	}

	c.dataMu.Lock()
	c.configHash = hash
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	for k, v := range defaultConfigMapData {
		// user left a value out in their config map, we want to ensure we default it
		if _, found := configMap.Data[k]; !found {
			configMap.Data[k] = v
			logging.FromContext(c.ctx).Infof("applying default config value %s=%s", k, v)
		}
	}

	for k, v := range configMap.Data {
		switch k {
		case paramBatchMaxDuration:
			c.batchMaxDuration = c.parsePositiveDuration(k, v, defaultConfigMapData[k])
		case paramBatchIdleDuration:
			c.batchIdleDuration = c.parsePositiveDuration(k, v, defaultConfigMapData[k])
		default:
			logging.FromContext(c.ctx).Warnf("ignoring unknown config parameter %s", k)
		}
	}
	c.dataMu.Unlock()
	// notify watchers
	c.watcherMu.Lock()
	defer c.watcherMu.Unlock()
	for _, w := range c.watchers {
		w(c)
	}
}

func (c *config) parsePositiveDuration(configKey, configValue string, defaultValue string) time.Duration {
	duration, err := time.ParseDuration(configValue)
	if err != nil {
		logging.FromContext(c.ctx).Errorf("unable to parse %s value %q: %s, using default value of %s", configKey, configValue, err, defaultValue)
	} else if duration < 0 {
		logging.FromContext(c.ctx).Errorf("negative values not allowed for %s, using default value of %s", configKey, defaultValue)
		duration = 0
	}
	if duration == 0 {
		duration, err = time.ParseDuration(defaultValue)
		if err != nil {
			// shouldn't occur, but just in case
			logging.FromContext(c.ctx).Errorf("parsing default value %s for key %s, %s", configValue, configKey, err)
		}
	}
	return duration
}
