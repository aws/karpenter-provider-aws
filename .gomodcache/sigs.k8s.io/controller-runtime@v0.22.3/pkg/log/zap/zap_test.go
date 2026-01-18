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

package zap

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// testStringer is a fmt.Stringer.
type testStringer struct{}

func (testStringer) String() string {
	return "value"
}

// fakeSyncWriter is a fake zap.SyncerWriter that lets us test if sync was called.
type fakeSyncWriter bool

func (w *fakeSyncWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *fakeSyncWriter) Sync() error {
	*w = true
	return nil
}

// logInfo is the information for a particular fakeLogger message.
type logInfo struct {
	name []string
	tags []interface{}
	msg  string
}

// fakeLoggerRoot is the root object to which all fakeLoggers record their messages.
type fakeLoggerRoot struct {
	messages []logInfo
}

var _ logr.LogSink = &fakeLogger{}

// fakeLogger is a fake implementation of logr.Logger that records
// messages, tags, and names,
// just records the name.
type fakeLogger struct {
	name []string
	tags []interface{}

	root *fakeLoggerRoot
}

func (f *fakeLogger) Init(info logr.RuntimeInfo) {
}

func (f *fakeLogger) WithName(name string) logr.LogSink {
	names := append([]string(nil), f.name...)
	names = append(names, name)
	return &fakeLogger{
		name: names,
		tags: f.tags,
		root: f.root,
	}
}

func (f *fakeLogger) WithValues(vals ...interface{}) logr.LogSink {
	tags := append([]interface{}(nil), f.tags...)
	tags = append(tags, vals...)
	return &fakeLogger{
		name: f.name,
		tags: tags,
		root: f.root,
	}
}

func (f *fakeLogger) Error(err error, msg string, vals ...interface{}) {
	tags := append([]interface{}(nil), f.tags...)
	tags = append(tags, "error", err)
	tags = append(tags, vals...)
	f.root.messages = append(f.root.messages, logInfo{
		name: append([]string(nil), f.name...),
		tags: tags,
		msg:  msg,
	})
}

func (f *fakeLogger) Info(level int, msg string, vals ...interface{}) {
	tags := append([]interface{}(nil), f.tags...)
	tags = append(tags, vals...)
	f.root.messages = append(f.root.messages, logInfo{
		name: append([]string(nil), f.name...),
		tags: tags,
		msg:  msg,
	})
}

func (f *fakeLogger) Enabled(level int) bool { return true }
func (f *fakeLogger) V(lvl int) logr.LogSink { return f }

var _ = Describe("Zap options setup", func() {
	var opts *Options

	BeforeEach(func() {
		opts = &Options{}
	})

	It("should enable development mode", func() {
		UseDevMode(true)(opts)
		Expect(opts.Development).To(BeTrue())
	})

	It("should disable development mode", func() {
		UseDevMode(false)(opts)
		Expect(opts.Development).To(BeFalse())
	})

	It("should set a custom writer", func() {
		var w fakeSyncWriter
		WriteTo(&w)(opts)
		Expect(opts.DestWriter).To(Equal(&w))
	})
})

var _ = Describe("Zap logger setup", func() {
	Context("when logging kubernetes objects", func() {
		var logOut *bytes.Buffer
		var logger logr.Logger

		defineTests := func() {
			It("should log a standard namespaced Kubernetes object name and namespace", func() {
				pod := &corev1.Pod{}
				pod.Name = "some-pod"
				pod.Namespace = "some-ns"
				logger.Info("here's a kubernetes object", "thing", pod)

				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", map[string]interface{}{
					"name":      pod.Name,
					"namespace": pod.Namespace,
				}))
			})

			It("should work fine with normal stringers", func() {
				logger.Info("here's a non-kubernetes stringer", "thing", testStringer{})
				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", "value"))
			})

			It("should log a standard non-namespaced Kubernetes object name", func() {
				node := &corev1.Node{}
				node.Name = "some-node"
				logger.Info("here's a kubernetes object", "thing", node)

				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", map[string]interface{}{
					"name": node.Name,
				}))
			})

			It("should log a standard Kubernetes object's kind, if set", func() {
				node := &corev1.Node{}
				node.Name = "some-node"
				node.APIVersion = "v1"
				node.Kind = "Node"
				logger.Info("here's a kubernetes object", "thing", node)

				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", map[string]interface{}{
					"name":       node.Name,
					"apiVersion": "v1",
					"kind":       "Node",
				}))
			})

			It("should log a standard non-namespaced NamespacedName name", func() {
				name := types.NamespacedName{Name: "some-node"}
				logger.Info("here's a kubernetes object", "thing", name)

				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", map[string]interface{}{
					"name": name.Name,
				}))
			})

			It("should log an unstructured Kubernetes object", func() {
				pod := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"name":      "some-pod",
							"namespace": "some-ns",
						},
					},
				}
				logger.Info("here's a kubernetes object", "thing", pod)

				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", map[string]interface{}{
					"name":      "some-pod",
					"namespace": "some-ns",
				}))
			})

			It("should log a standard namespaced NamespacedName name and namespace", func() {
				name := types.NamespacedName{Name: "some-pod", Namespace: "some-ns"}
				logger.Info("here's a kubernetes object", "thing", name)

				outRaw := logOut.Bytes()
				res := map[string]interface{}{}
				Expect(json.Unmarshal(outRaw, &res)).To(Succeed())

				Expect(res).To(HaveKeyWithValue("thing", map[string]interface{}{
					"name":      name.Name,
					"namespace": name.Namespace,
				}))
			})

			It("should not panic with nil obj", func() {
				var pod *corev1.Pod
				logger.Info("here's a kubernetes object", "thing", pod)

				outRaw := logOut.Bytes()
				Expect(string(outRaw)).Should(ContainSubstring("got nil for runtime.Object"))
			})
		}

		Context("with logger created using New", func() {
			BeforeEach(func() {
				logOut = new(bytes.Buffer)
				By("setting up the logger")
				// use production settings (false) to get just json output
				logger = New(WriteTo(logOut), UseDevMode(false))
			})
			defineTests()
		})
	})
})

var _ = Describe("Zap log level flag options setup", func() {
	var (
		fromFlags      Options
		fs             flag.FlagSet
		logInfoLevel0  = "info text"
		logDebugLevel1 = "debug 1 text"
		logDebugLevel2 = "debug 2 text"
		logDebugLevel3 = "debug 3 text"
	)

	BeforeEach(func() {
		fromFlags = Options{}
		fs = *flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	})

	Context("with  zap-log-level options provided", func() {
		It("Should output logs for info and debug zap-log-level.", func() {
			args := []string{"--zap-log-level=debug"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)

			outRaw := logOut.Bytes()

			Expect(string(outRaw)).Should(ContainSubstring(logInfoLevel0))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel1))
		})

		It("Should output only error logs, otherwise empty logs", func() {
			args := []string{"--zap-log-level=error"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())

			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)

			outRaw := logOut.Bytes()

			Expect(outRaw).To(BeEmpty())
		})

		It("Should output only panic logs, otherwise empty logs", func() {
			args := []string{"--zap-log-level=panic"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())

			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)
			logger.V(2).Info(logDebugLevel2)

			outRaw := logOut.Bytes()

			Expect(outRaw).To(BeEmpty())
		})
	})

	Context("with  zap-log-level  with increased verbosity.", func() {
		It("Should output debug and info log, with default production mode.", func() {
			args := []string{"--zap-log-level=1"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)

			outRaw := logOut.Bytes()

			Expect(string(outRaw)).Should(ContainSubstring(logInfoLevel0))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel1))
		})

		It("Should output info and debug logs, with development mode.", func() {
			args := []string{"--zap-log-level=1", "--zap-devel=true"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)

			outRaw := logOut.Bytes()

			Expect(string(outRaw)).Should(ContainSubstring(logInfoLevel0))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel1))
		})

		It("Should output info, and debug logs with increased verbosity, and with development mode set to true.", func() {
			args := []string{"--zap-log-level=3", "--zap-devel=false"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)
			logger.V(2).Info(logDebugLevel2)
			logger.V(3).Info(logDebugLevel3)

			outRaw := logOut.Bytes()

			Expect(string(outRaw)).Should(ContainSubstring(logInfoLevel0))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel1))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel2))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel3))
		})
		It("Should output info, and debug logs with increased verbosity, and with production mode set to true.", func() {
			args := []string{"--zap-log-level=3", "--zap-devel=true"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.V(0).Info(logInfoLevel0)
			logger.V(1).Info(logDebugLevel1)
			logger.V(2).Info(logDebugLevel2)
			logger.V(3).Info(logDebugLevel3)

			outRaw := logOut.Bytes()

			Expect(string(outRaw)).Should(ContainSubstring(logInfoLevel0))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel1))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel2))
			Expect(string(outRaw)).Should(ContainSubstring(logDebugLevel3))
		})
	})

	Context("with  zap-stacktrace-level options provided", func() {
		It("Should output stacktrace at info level, with development mode set to true.", func() {
			args := []string{"--zap-stacktrace-level=info", "--zap-devel=true"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			out := Options{}
			UseFlagOptions(&fromFlags)(&out)

			Expect(out.StacktraceLevel.Enabled(zapcore.InfoLevel)).To(BeTrue())
		})

		It("Should output stacktrace at error level, with development mode set to true.", func() {
			args := []string{"--zap-stacktrace-level=error", "--zap-devel=true"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			out := Options{}
			UseFlagOptions(&fromFlags)(&out)

			Expect(out.StacktraceLevel.Enabled(zapcore.ErrorLevel)).To(BeTrue())
		})

		It("Should output stacktrace at panic level, with development mode set to true.", func() {
			args := []string{"--zap-stacktrace-level=panic", "--zap-devel=true"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			out := Options{}
			UseFlagOptions(&fromFlags)(&out)

			Expect(out.StacktraceLevel.Enabled(zapcore.PanicLevel)).To(BeTrue())
			Expect(out.StacktraceLevel.Enabled(zapcore.ErrorLevel)).To(BeFalse())
			Expect(out.StacktraceLevel.Enabled(zapcore.InfoLevel)).To(BeFalse())
		})
	})

	Context("with only -zap-devel flag provided", func() {
		It("Should set dev=true.", func() {
			args := []string{"--zap-devel=true"}
			fromFlags.BindFlags(&fs)
			if err := fs.Parse(args); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}
			out := Options{}
			UseFlagOptions(&fromFlags)(&out)

			Expect(out.Development).To(BeTrue())
			Expect(out.Encoder).To(BeNil())
			Expect(out.Level).To(BeNil())
			Expect(out.StacktraceLevel).To(BeNil())
			Expect(out.EncoderConfigOptions).To(BeNil())
		})
		It("Should set dev=false", func() {
			args := []string{"--zap-devel=false"}
			fromFlags.BindFlags(&fs)
			if err := fs.Parse(args); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}
			out := Options{}
			UseFlagOptions(&fromFlags)(&out)

			Expect(out.Development).To(BeFalse())
			Expect(out.Encoder).To(BeNil())
			Expect(out.Level).To(BeNil())
			Expect(out.StacktraceLevel).To(BeNil())
			Expect(out.EncoderConfigOptions).To(BeNil())
		})
	})

	Context("with zap-time-encoding flag provided", func() {
		It("Should set time encoder in options", func() {
			args := []string{"--zap-time-encoding=rfc3339"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())

			opt := Options{}
			UseFlagOptions(&fromFlags)(&opt)
			opt.addDefaults()

			optVal := reflect.ValueOf(opt.TimeEncoder)
			expVal := reflect.ValueOf(zapcore.RFC3339TimeEncoder)

			Expect(optVal.Pointer()).To(Equal(expVal.Pointer()))
		})

		It("Should default to 'rfc3339' time encoding", func() {
			args := []string{""}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())

			opt := Options{}
			UseFlagOptions(&fromFlags)(&opt)
			opt.addDefaults()

			optVal := reflect.ValueOf(opt.TimeEncoder)
			expVal := reflect.ValueOf(zapcore.RFC3339TimeEncoder)

			Expect(optVal.Pointer()).To(Equal(expVal.Pointer()))
		})

		It("Should return an error message, with unknown time-encoding", func() {
			fs = *flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			args := []string{"--zap-time-encoding=foobar"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).To(HaveOccurred())
		})

		It("Should propagate time encoder to logger", func() {
			// zaps ISO8601TimeEncoder uses 2006-01-02T15:04:05.000Z0700 as pattern for iso8601 encoding
			iso8601Pattern := `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}.[0-9]{3}([-+][0-9]{4}|Z)`

			args := []string{"--zap-time-encoding=iso8601"}
			fromFlags.BindFlags(&fs)
			err := fs.Parse(args)
			Expect(err).ToNot(HaveOccurred())
			logOut := new(bytes.Buffer)

			logger := New(UseFlagOptions(&fromFlags), WriteTo(logOut))
			logger.Info("This is a test message")

			outRaw := logOut.Bytes()

			res := map[string]interface{}{}
			Expect(json.Unmarshal(outRaw, &res)).To(Succeed())
			Expect(res["ts"]).Should(MatchRegexp(iso8601Pattern))
		})
	})

	Context("with encoder options provided programmatically", func() {
		It("Should set JSON Encoder, with given Millis TimeEncoder option, and MessageKey", func() {
			logOut := new(bytes.Buffer)
			f := func(ec *zapcore.EncoderConfig) {
				ec.MessageKey = "MillisTimeFormat"
			}
			opts := func(o *Options) {
				o.EncoderConfigOptions = append(o.EncoderConfigOptions, f)
			}
			log := New(UseDevMode(false), WriteTo(logOut), opts)
			log.Info("This is a test message")
			outRaw := logOut.Bytes()
			// Assert for JSON Encoder
			res := map[string]interface{}{}
			Expect(json.Unmarshal(outRaw, &res)).To(Succeed())
			// Assert for MessageKey
			Expect(string(outRaw)).Should(ContainSubstring("MillisTimeFormat"))
		})

		Context("using Level()", func() {
			var logOut *bytes.Buffer

			BeforeEach(func() {
				logOut = new(bytes.Buffer)
			})

			It("logs with negative logr level", func() {
				By("setting up the logger")
				logger := New(WriteTo(logOut), Level(zapcore.Level(-3)))
				logger.V(3).Info("test 3") // Should be logged
				Expect(logOut.String()).To(ContainSubstring(`"msg":"test 3"`))
				logOut.Truncate(0)
				logger.V(1).Info("test 1") // Should be logged
				Expect(logOut.String()).To(ContainSubstring(`"msg":"test 1"`))
				logOut.Truncate(0)
				logger.V(4).Info("test 4") // Should not be logged
				Expect(logOut.String()).To(BeEmpty())
				logger.V(-3).Info("test -3")
				Expect(logOut.String()).To(ContainSubstring("test -3"))
			})
			It("does not log with positive logr level", func() {
				By("setting up the logger")
				logger := New(WriteTo(logOut), Level(zapcore.Level(1)))
				logger.V(1).Info("test 1")
				Expect(logOut.String()).To(BeEmpty())
				logger.V(3).Info("test 3")
				Expect(logOut.String()).To(BeEmpty())
			})
		})
	})
})
