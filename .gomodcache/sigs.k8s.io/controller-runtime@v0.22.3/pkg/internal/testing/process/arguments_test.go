/*
Copyright 2021 The Kubernetes Authors.

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

package process_test

import (
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "sigs.k8s.io/controller-runtime/pkg/internal/testing/process"
)

var _ = Describe("Arguments Templates", func() {
	It("templates URLs", func() {
		templates := []string{
			"plain URL: {{ .SomeURL }}",
			"method on URL: {{ .SomeURL.Hostname }}",
			"empty URL: {{ .EmptyURL }}",
			"handled empty URL: {{- if .EmptyURL }}{{ .EmptyURL }}{{ end }}",
		}
		data := struct {
			SomeURL  *url.URL
			EmptyURL *url.URL
		}{
			&url.URL{Scheme: "https", Host: "the.host.name:3456"},
			nil,
		}

		out, err := RenderTemplates(templates, data)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEquivalentTo([]string{
			"plain URL: https://the.host.name:3456",
			"method on URL: the.host.name",
			"empty URL: &lt;nil&gt;",
			"handled empty URL:",
		}))
	})

	It("templates strings", func() {
		templates := []string{
			"a string: {{ .SomeString }}",
			"empty string: {{- .EmptyString }}",
		}
		data := struct {
			SomeString  string
			EmptyString string
		}{
			"this is some random string",
			"",
		}

		out, err := RenderTemplates(templates, data)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEquivalentTo([]string{
			"a string: this is some random string",
			"empty string:",
		}))
	})

	It("has no access to unexported fields", func() {
		templates := []string{
			"this is just a string",
			"this blows up {{ .test }}",
		}
		data := struct{ test string }{"ooops private"}

		out, err := RenderTemplates(templates, data)
		Expect(out).To(BeEmpty())
		Expect(err).To(MatchError(
			ContainSubstring("is an unexported field of struct"),
		))
	})

	It("errors when field cannot be found", func() {
		templates := []string{"this does {{ .NotExist }}"}
		data := struct{ Unused string }{"unused"}

		out, err := RenderTemplates(templates, data)
		Expect(out).To(BeEmpty())
		Expect(err).To(MatchError(
			ContainSubstring("can't evaluate field"),
		))
	})

	Context("when joining with structured Arguments", func() {
		var (
			args  *Arguments
			templ = []string{
				"--cheese=parmesean",
				"-om",
				"nom nom nom",
				"--sharpness={{ .sharpness }}",
			}
			data = TemplateDefaults{
				Data: map[string]string{"sharpness": "extra"},
				Defaults: map[string][]string{
					"cracker": {"ritz"},
					"pickle":  {"kosher-dill"},
				},
				MinimalDefaults: map[string][]string{
					"pickle": {"kosher-dill"},
				},
			}
		)
		BeforeEach(func() {
			args = EmptyArguments()
		})

		Context("when a template is given", func() {
			It("should use minimal defaults", func() {
				all, _, err := TemplateAndArguments(templ, args, data)
				Expect(err).NotTo(HaveOccurred())
				Expect(all).To(SatisfyAll(
					Not(ContainElement("--cracker=ritz")),
					ContainElement("--pickle=kosher-dill"),
				))
			})

			It("should render the template against the data", func() {
				all, _, err := TemplateAndArguments(templ, args, data)
				Expect(err).NotTo(HaveOccurred())
				Expect(all).To(ContainElements(
					"--sharpness=extra",
				))
			})

			It("should append the rendered template to structured arguments", func() {
				args.Append("cheese", "cheddar")

				all, _, err := TemplateAndArguments(templ, args, data)
				Expect(err).NotTo(HaveOccurred())
				Expect(all).To(Equal([]string{
					"--cheese=cheddar",
					"--cheese=parmesean",
					"--pickle=kosher-dill",
					"--sharpness=extra",
					"-om",
					"nom nom nom",
				}))
			})

			It("should indicate which arguments were not able to be converted to structured flags", func() {
				_, rest, err := TemplateAndArguments(templ, args, data)
				Expect(err).NotTo(HaveOccurred())
				Expect(rest).To(Equal([]string{"-om", "nom nom nom"}))

			})
		})

		Context("when no template is given", func() {
			It("should render the structured arguments with the given defaults", func() {
				args.
					Append("cheese", "cheddar", "parmesean").
					Append("cracker", "triscuit")

				Expect(TemplateAndArguments(nil, args, data)).To(Equal([]string{
					"--cheese=cheddar",
					"--cheese=parmesean",
					"--cracker=ritz",
					"--cracker=triscuit",
					"--pickle=kosher-dill",
				}))
			})
		})
	})

	Context("when converting to structured Arguments", func() {
		var args *Arguments
		BeforeEach(func() {
			args = EmptyArguments()
		})

		It("should skip arguments that don't start with `--`", func() {
			rest := SliceToArguments([]string{"-first", "second", "--foo=bar"}, args)
			Expect(rest).To(Equal([]string{"-first", "second"}))
			Expect(args.AsStrings(nil)).To(Equal([]string{"--foo=bar"}))
		})

		It("should skip arguments that don't contain an `=` because they're ambiguous", func() {
			rest := SliceToArguments([]string{"--first", "--second", "--foo=bar"}, args)
			Expect(rest).To(Equal([]string{"--first", "--second"}))
			Expect(args.AsStrings(nil)).To(Equal([]string{"--foo=bar"}))
		})

		It("should stop at the flag terminator (`--`)", func() {
			rest := SliceToArguments([]string{"--first", "--second", "--", "--foo=bar"}, args)
			Expect(rest).To(Equal([]string{"--first", "--second", "--", "--foo=bar"}))
			Expect(args.AsStrings(nil)).To(BeEmpty())
		})

		It("should split --foo=bar into Append(foo, bar)", func() {
			rest := SliceToArguments([]string{"--foo=bar1", "--foo=bar2"}, args)
			Expect(rest).To(BeEmpty())
			Expect(args.Get("foo").Get(nil)).To(Equal([]string{"bar1", "bar2"}))
		})

		It("should split --foo=bar=baz into Append(foo, bar=baz)", func() {
			rest := SliceToArguments([]string{"--vmodule=file.go=3", "--vmodule=other.go=4"}, args)
			Expect(rest).To(BeEmpty())
			Expect(args.Get("vmodule").Get(nil)).To(Equal([]string{"file.go=3", "other.go=4"}))
		})

		It("should append to existing arguments", func() {
			args.Append("foo", "barA")
			rest := SliceToArguments([]string{"--foo=bar1", "--foo=bar2"}, args)
			Expect(rest).To(BeEmpty())
			Expect(args.Get("foo").Get([]string{"barI"})).To(Equal([]string{"barI", "barA", "bar1", "bar2"}))
		})
	})
})

var _ = Describe("Arguments", func() {
	Context("when appending", func() {
		It("should copy from defaults when appending for the first time", func() {
			args := EmptyArguments().
				Append("some-key", "val3")
			Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"val1", "val2", "val3"}))
		})

		It("should not copy from defaults if the flag has been disabled previously", func() {
			args := EmptyArguments().
				Disable("some-key").
				Append("some-key", "val3")
			Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"val3"}))
		})

		It("should only copy defaults the first time", func() {
			args := EmptyArguments().
				Append("some-key", "val3", "val4").
				Append("some-key", "val5")
			Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"val1", "val2", "val3", "val4", "val5"}))
		})

		It("should not copy from defaults if the flag has been previously overridden", func() {
			args := EmptyArguments().
				Set("some-key", "vala").
				Append("some-key", "valb", "valc")
			Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"vala", "valb", "valc"}))
		})

		Context("when explicitly overriding defaults", func() {
			It("should not copy from defaults, but should append to previous calls", func() {
				args := EmptyArguments().
					AppendNoDefaults("some-key", "vala").
					AppendNoDefaults("some-key", "valb", "valc")
				Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"vala", "valb", "valc"}))
			})

			It("should not copy from defaults, but should respect previous appends' copies", func() {
				args := EmptyArguments().
					Append("some-key", "vala").
					AppendNoDefaults("some-key", "valb", "valc")
				Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"val1", "val2", "vala", "valb", "valc"}))
			})

			It("should not copy from defaults if the flag has been previously appended to ignoring defaults", func() {
				args := EmptyArguments().
					AppendNoDefaults("some-key", "vala").
					Append("some-key", "valb", "valc")
				Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"vala", "valb", "valc"}))
			})
		})
	})

	It("should ignore defaults when overriding", func() {
		args := EmptyArguments().
			Set("some-key", "vala")
		Expect(args.Get("some-key").Get([]string{"val1", "val2"})).To(Equal([]string{"vala"}))
	})

	It("should allow directly setting the argument value for custom argument types", func() {
		args := EmptyArguments().
			SetRaw("custom-key", commaArg{"val3"}).
			Append("custom-key", "val4")
		Expect(args.Get("custom-key").Get([]string{"val1", "val2"})).To(Equal([]string{"val1,val2,val3,val4"}))
	})

	Context("when rendering flags", func() {
		It("should not render defaults for disabled flags", func() {
			defs := map[string][]string{
				"some-key":  {"val1", "val2"},
				"other-key": {"val"},
			}
			args := EmptyArguments().
				Disable("some-key")
			Expect(args.AsStrings(defs)).To(ConsistOf("--other-key=val"))
		})

		It("should render name-only flags as --key", func() {
			args := EmptyArguments().
				Enable("some-key")
			Expect(args.AsStrings(nil)).To(ConsistOf("--some-key"))
		})

		It("should render multiple values as --key=val1, --key=val2", func() {
			args := EmptyArguments().
				Append("some-key", "val1", "val2").
				Append("other-key", "vala", "valb")
			Expect(args.AsStrings(nil)).To(ConsistOf("--other-key=valb", "--other-key=vala", "--some-key=val1", "--some-key=val2"))
		})

		It("should read from defaults if the user hasn't set a value for a flag", func() {
			defs := map[string][]string{
				"some-key": {"val1", "val2"},
			}
			args := EmptyArguments().
				Append("other-key", "vala", "valb")
			Expect(args.AsStrings(defs)).To(ConsistOf("--other-key=valb", "--other-key=vala", "--some-key=val1", "--some-key=val2"))
		})

		It("should not render defaults if the user has set a value for a flag", func() {
			defs := map[string][]string{
				"some-key": {"val1", "val2"},
			}
			args := EmptyArguments().
				Set("some-key", "vala")
			Expect(args.AsStrings(defs)).To(ConsistOf("--some-key=vala"))
		})
	})
})

type commaArg []string

func (a commaArg) Get(defs []string) []string {
	// not quite, but close enough
	return []string{strings.Join(defs, ",") + "," + strings.Join(a, ",")}
}
func (a commaArg) Append(vals ...string) Arg {
	return commaArg(append(a, vals...)) //nolint:unconvert
}
