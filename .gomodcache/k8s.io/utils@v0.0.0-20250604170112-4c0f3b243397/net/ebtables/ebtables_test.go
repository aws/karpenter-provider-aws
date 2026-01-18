/*
Copyright 2016 The Kubernetes Authors.

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

package ebtables

import (
	"strings"
	"testing"

	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

func TestEnsureChain(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeAction{
			// Does not Exists
			func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 1} },
			// Success
			func() ([]byte, []byte, error) { return []byte{}, nil, nil },
			// Exists
			func() ([]byte, []byte, error) { return nil, nil, nil },
			// Does not Exists
			func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 1} },
			// Fail to create chain
			func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 2} },
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
	}

	runner := New(&fexec)
	exists, err := runner.EnsureChain(TableFilter, "TEST-CHAIN")
	if exists {
		t.Errorf("expected exists = false")
	}
	if err != nil {
		t.Errorf("expected err = nil")
	}

	exists, err = runner.EnsureChain(TableFilter, "TEST-CHAIN")
	if !exists {
		t.Errorf("expected exists = true")
	}
	if err != nil {
		t.Errorf("expected err = nil")
	}

	exists, err = runner.EnsureChain(TableFilter, "TEST-CHAIN")
	if exists {
		t.Errorf("expected exists = false")
	}
	errStr := "Failed to ensure TEST-CHAIN chain: exit 2, output:"
	if err == nil || !strings.Contains(err.Error(), errStr) {
		t.Errorf("expected error: %q", errStr)
	}
}

func TestEnsureRule(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeAction{
			// Exists
			func() ([]byte, []byte, error) {
				return []byte(`Bridge table: filter

Bridge chain: OUTPUT, entries: 4, policy: ACCEPT
-j TEST
`), nil, nil
			},
			// Does not Exists.
			func() ([]byte, []byte, error) {
				return []byte(`Bridge table: filter

Bridge chain: TEST, entries: 0, policy: ACCEPT`), nil, nil
			},
			// Fail to create
			func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 2} },
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
	}

	runner := New(&fexec)

	exists, err := runner.EnsureRule(Append, TableFilter, ChainOutput, "-j", "TEST")
	if !exists {
		t.Errorf("expected exists = true")
	}
	if err != nil {
		t.Errorf("expected err = nil")
	}

	exists, err = runner.EnsureRule(Append, TableFilter, ChainOutput, "-j", "NEXT-TEST")
	if exists {
		t.Errorf("expected exists = false")
	}
	errStr := "Failed to ensure rule: exit 2, output: "
	if err == nil || err.Error() != errStr {
		t.Errorf("expected error: %q", errStr)
	}
}

func TestDeleteRule(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeAction{
			// Exists
			func() ([]byte, []byte, error) {
				return []byte(`Bridge table: filter

Bridge chain: OUTPUT, entries: 4, policy: ACCEPT
-j TEST
`), nil, nil
			},
			// Fail to delete
			func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 2} },
			// Does not Exists.
			func() ([]byte, []byte, error) {
				return []byte(`Bridge table: filter

Bridge chain: TEST, entries: 0, policy: ACCEPT`), nil, nil
			},
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
	}

	runner := New(&fexec)

	err := runner.DeleteRule(TableFilter, ChainOutput, "-j", "TEST")
	errStr := "Failed to delete rule: exit 2, output: "
	if err == nil || err.Error() != errStr {
		t.Errorf("expected error: %q", errStr)
	}

	err = runner.DeleteRule(TableFilter, ChainOutput, "-j", "TEST")
	if err != nil {
		t.Errorf("expected err = nil")
	}
}

func Test_parseVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{
			name:    "version starting with `v`",
			version: "v2.0.10",
			want:    "2.0.10",
			wantErr: false,
		},
		{
			name:    "version without containing `v`",
			version: "2.0.10",
			want:    "2.0.10",
			wantErr: false,
		},
		{
			name:    "version containing `v` in between the regex expression match",
			version: "2.0v.10",
			want:    "",
			wantErr: true,
		},
		{
			name:    "version containing `v` after the regex expression match",
			version: "2.0.10v",
			want:    "2.0.10",
			wantErr: false,
		},
		{
			name:    "version starting with `v` and containing a symbol in between",
			version: "v2.0.10-4",
			want:    "2.0.10",
			wantErr: false,
		},
		{
			name:    "version starting with `v` and containing a symbol/alphabets in between",
			version: "v2.0a.10-4",
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}
