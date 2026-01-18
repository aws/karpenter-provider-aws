package aws

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkrand "github.com/aws/aws-sdk-go-v2/internal/rand"
	"github.com/aws/aws-sdk-go-v2/internal/sdk"
)

type stubCredentialsProvider struct {
	creds   Credentials
	expires time.Time
	err     error

	onInvalidate func(*stubCredentialsProvider)
}

func (s *stubCredentialsProvider) Retrieve(ctx context.Context) (Credentials, error) {
	creds := s.creds
	creds.Source = "stubCredentialsProvider"
	creds.CanExpire = !s.expires.IsZero()
	creds.Expires = s.expires

	return creds, s.err
}

func (s *stubCredentialsProvider) Invalidate() {
	s.onInvalidate(s)
}

func TestCredentialsCache_Cache(t *testing.T) {
	expect := Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		CanExpire:       true,
		Expires:         time.Now().Add(10 * time.Minute),
	}

	var called bool
	p := NewCredentialsCache(CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
		if called {
			t.Fatalf("expect provider.Retrieve to only be called once")
		}
		called = true
		return expect, nil
	}))

	for i := 0; i < 2; i++ {
		creds, err := p.Retrieve(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
		if e, a := expect, creds; e != a {
			t.Errorf("expect %v credential, got %v", e, a)
		}
	}
}

func TestCredentialsCache_Expires(t *testing.T) {
	orig := sdk.NowTime
	defer func() { sdk.NowTime = orig }()
	var mockTime time.Time
	sdk.NowTime = func() time.Time { return mockTime }

	cases := []struct {
		Creds  func() Credentials
		Called int
	}{
		{
			Called: 2,
			Creds: func() Credentials {
				return Credentials{
					AccessKeyID:     "key",
					SecretAccessKey: "secret",
					CanExpire:       true,
					Expires:         mockTime.Add(5),
				}
			},
		},
		{
			Called: 1,
			Creds: func() Credentials {
				return Credentials{
					AccessKeyID:     "key",
					SecretAccessKey: "secret",
				}
			},
		},
		{
			Called: 6,
			Creds: func() Credentials {
				return Credentials{
					AccessKeyID:     "key",
					SecretAccessKey: "secret",
					CanExpire:       true,
					Expires:         mockTime,
				}
			},
		},
	}

	for _, c := range cases {
		var called int
		p := NewCredentialsCache(CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
			called++
			return c.Creds(), nil
		}))

		p.Retrieve(context.Background())
		p.Retrieve(context.Background())
		p.Retrieve(context.Background())

		mockTime = mockTime.Add(10)

		p.Retrieve(context.Background())
		p.Retrieve(context.Background())
		p.Retrieve(context.Background())

		if e, a := c.Called, called; e != a {
			t.Errorf("expect %v called, got %v", e, a)
		}
	}
}

func TestCredentialsCache_ExpireTime(t *testing.T) {
	orig := sdk.NowTime
	defer func() { sdk.NowTime = orig }()
	var mockTime time.Time
	sdk.NowTime = func() time.Time { return mockTime }

	cases := map[string]struct {
		ExpireTime   time.Time
		ExpiryWindow time.Duration
		JitterFrac   float64
		Validate     func(t *testing.T, v time.Time)
	}{
		"no expire window": {
			Validate: func(t *testing.T, v time.Time) {
				t.Helper()
				if e, a := mockTime, v; !e.Equal(a) {
					t.Errorf("expect %v, got %v", e, a)
				}
			},
		},
		"expire window": {
			ExpireTime:   mockTime.Add(100),
			ExpiryWindow: 50,
			Validate: func(t *testing.T, v time.Time) {
				t.Helper()
				if e, a := mockTime.Add(50), v; !e.Equal(a) {
					t.Errorf("expect %v, got %v", e, a)
				}
			},
		},
		"expire window with jitter": {
			ExpireTime:   mockTime.Add(100),
			JitterFrac:   0.5,
			ExpiryWindow: 50,
			Validate: func(t *testing.T, v time.Time) {
				t.Helper()
				max := mockTime.Add(75)
				min := mockTime.Add(50)
				if v.Before(min) {
					t.Errorf("expect %v to be before %s", v, min)
				}
				if v.After(max) {
					t.Errorf("expect %v to be after %s", v, max)
				}
			},
		},
		"no expire window with jitter": {
			ExpireTime: mockTime,
			JitterFrac: 0.5,
			Validate: func(t *testing.T, v time.Time) {
				t.Helper()
				if e, a := mockTime, v; !e.Equal(a) {
					t.Errorf("expect %v, got %v", e, a)
				}
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			p := NewCredentialsCache(CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
				return Credentials{
					AccessKeyID:     "accessKey",
					SecretAccessKey: "secretKey",
					CanExpire:       true,
					Expires:         tt.ExpireTime,
				}, nil
			}), func(options *CredentialsCacheOptions) {
				options.ExpiryWindow = tt.ExpiryWindow
				options.ExpiryWindowJitterFrac = tt.JitterFrac
			})

			credentials, err := p.Retrieve(context.Background())
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			tt.Validate(t, credentials.Expires)
		})
	}
}

func TestCredentialsCache_Error(t *testing.T) {
	p := NewCredentialsCache(CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
		return Credentials{}, fmt.Errorf("failed")
	}))

	creds, err := p.Retrieve(context.Background())
	if err == nil {
		t.Fatalf("expect error, not none")
	}
	if e, a := "failed", err.Error(); !strings.Contains(a, e) {
		t.Errorf("expect %q, got %q", e, a)
	}
	if e, a := (Credentials{}), creds; e != a {
		t.Errorf("expect empty credentials, got %v", a)
	}
}

func TestCredentialsCache_Race(t *testing.T) {
	expect := Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
	}
	var called bool
	p := NewCredentialsCache(CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
		if called {
			t.Fatalf("expect provider.Retrieve only called once")
		}
		called = true
		return expect, nil
	}))

	var wg sync.WaitGroup
	wg.Add(100)

	for i := 0; i < 100; i++ {
		go func() {
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			creds, err := p.Retrieve(context.Background())
			if err != nil {
				t.Errorf("expect no error, got %v", err)
			}
			if e, a := expect, creds; e != a {
				t.Errorf("expect %v, got %v", e, a)
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

type stubConcurrentProvider struct {
	called uint32
	done   chan struct{}
}

func (s *stubConcurrentProvider) Retrieve(ctx context.Context) (Credentials, error) {
	atomic.AddUint32(&s.called, 1)
	<-s.done
	return Credentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}, nil
}

func TestCredentialsCache_RetrieveConcurrent(t *testing.T) {
	stub := &stubConcurrentProvider{
		done: make(chan struct{}),
	}
	provider := NewCredentialsCache(stub)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			provider.Retrieve(context.Background())
			wg.Done()
		}()
	}

	// Validates that a single call to Retrieve is shared between two calls to
	// Retrieve method call.
	stub.done <- struct{}{}
	wg.Wait()

	if e, a := uint32(1), atomic.LoadUint32(&stub.called); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestCredentialsCache_cacheStrategies(t *testing.T) {
	origSdkTime := sdk.NowTime
	defer func() { sdk.NowTime = origSdkTime }()
	sdk.NowTime = func() time.Time {
		return time.Date(2015, 4, 8, 0, 0, 0, 0, time.UTC)
	}

	origSdkRandReader := sdkrand.Reader
	defer func() { sdkrand.Reader = origSdkRandReader }()
	sdkrand.Reader = byteReader(0xFF)

	cases := map[string]struct {
		options      func(*CredentialsCacheOptions)
		provider     CredentialsProvider
		initialCreds Credentials
		expectErr    string
		expectCreds  Credentials
	}{
		"default": {
			provider: struct {
				mockProvider
			}{
				mockProvider: mockProvider{
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
				},
			},
			expectCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(time.Hour),
			},
		},
		"default with window": {
			options: func(o *CredentialsCacheOptions) {
				o.ExpiryWindow = 5 * time.Minute
			},
			provider: struct {
				mockProvider
			}{
				mockProvider: mockProvider{
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
				},
			},
			expectCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(55 * time.Minute),
			},
		},
		"default with window jitterFrac": {
			options: func(o *CredentialsCacheOptions) {
				o.ExpiryWindow = 5 * time.Minute
				o.ExpiryWindowJitterFrac = 0.5
			},
			provider: struct {
				mockProvider
			}{
				mockProvider: mockProvider{
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
				},
			},
			expectCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(57*time.Minute + 29*time.Second),
			},
		},
		"handle refresh": {
			initialCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(-time.Hour),
			},
			provider: struct {
				mockProvider
				mockHandleFailToRefresh
			}{
				mockProvider: mockProvider{
					err: fmt.Errorf("some error"),
				},
				mockHandleFailToRefresh: mockHandleFailToRefresh{
					expectInputCreds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(-time.Hour),
					},
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
				},
			},
			expectCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(time.Hour),
			},
		},
		"handle refresh error": {
			initialCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(-time.Hour),
			},
			provider: struct {
				mockProvider
				mockHandleFailToRefresh
			}{
				mockProvider: mockProvider{
					err: fmt.Errorf("some error"),
				},
				mockHandleFailToRefresh: mockHandleFailToRefresh{
					expectInputCreds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(-time.Hour),
					},
					expectErr: "some error",
					err:       fmt.Errorf("some other error"),
				},
			},
			expectErr: "some other error",
		},
		"adjust expires": {
			options: func(o *CredentialsCacheOptions) {
				o.ExpiryWindow = 5 * time.Minute
			},
			provider: struct {
				mockProvider
				mockAdjustExpiryBy
			}{
				mockProvider: mockProvider{
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
				},
				mockAdjustExpiryBy: mockAdjustExpiryBy{
					expectInputCreds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
					expectDur: -5 * time.Minute,
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(25 * time.Minute),
					},
				},
			},
			expectCreds: Credentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CanExpire:       true,
				Expires:         sdk.NowTime().Add(25 * time.Minute),
			},
		},
		"adjust expires error": {
			options: func(o *CredentialsCacheOptions) {
				o.ExpiryWindow = 5 * time.Minute
			},
			provider: struct {
				mockProvider
				mockAdjustExpiryBy
			}{
				mockProvider: mockProvider{
					creds: Credentials{
						AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
						SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						CanExpire:       true,
						Expires:         sdk.NowTime().Add(time.Hour),
					},
				},
				mockAdjustExpiryBy: mockAdjustExpiryBy{
					err: fmt.Errorf("some error"),
				},
			},
			expectErr: "some error",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var optFns []func(*CredentialsCacheOptions)
			if c.options != nil {
				optFns = append(optFns, c.options)
			}
			provider := NewCredentialsCache(c.provider, optFns...)

			if c.initialCreds.HasKeys() {
				creds := c.initialCreds
				provider.creds.Store(&creds)
			}

			creds, err := provider.Retrieve(context.Background())
			if err == nil && len(c.expectErr) != 0 {
				t.Fatalf("expect error %v, got none", c.expectErr)
			}
			if err != nil && len(c.expectErr) == 0 {
				t.Fatalf("expect no error, got %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), c.expectErr) {
				t.Fatalf("expect error to contain %v, got %v", c.expectErr, err)
			}
			if c.expectErr != "" {
				return
			}
			// Truncate expires time so its easy to compare
			creds.Expires = creds.Expires.Truncate(time.Second)

			if diff := cmpDiff(c.expectCreds, creds); diff != "" {
				t.Errorf("expect creds match\n%s", diff)
			}
		})
	}
}

type byteReader byte

func (b byteReader) Read(p []byte) (int, error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(b)
	}
	return len(p), nil
}

type mockProvider struct {
	creds Credentials
	err   error
}

var _ CredentialsProvider = mockProvider{}

func (m mockProvider) Retrieve(context.Context) (Credentials, error) {
	return m.creds, m.err
}

type mockHandleFailToRefresh struct {
	expectInputCreds Credentials
	expectErr        string
	creds            Credentials
	err              error
}

var _ HandleFailRefreshCredentialsCacheStrategy = mockHandleFailToRefresh{}

func (m mockHandleFailToRefresh) HandleFailToRefresh(ctx context.Context, prevCreds Credentials, err error) (
	Credentials, error,
) {
	if m.expectInputCreds.HasKeys() {
		if e, a := m.expectInputCreds, prevCreds; e != a {
			return Credentials{}, fmt.Errorf("expect %v creds, got %v", e, a)
		}
	}
	if m.expectErr != "" {
		if err == nil {
			return Credentials{}, fmt.Errorf("expect input error, got none")
		}
		if e, a := m.expectErr, err.Error(); !strings.Contains(a, e) {
			return Credentials{}, fmt.Errorf("expect %v in error, got %v", e, a)
		}
	}
	return m.creds, m.err
}

type mockAdjustExpiryBy struct {
	expectInputCreds Credentials
	expectDur        time.Duration
	creds            Credentials
	err              error
}

var _ AdjustExpiresByCredentialsCacheStrategy = mockAdjustExpiryBy{}

func (m mockAdjustExpiryBy) AdjustExpiresBy(creds Credentials, dur time.Duration) (
	Credentials, error,
) {
	if m.expectInputCreds.HasKeys() {
		if diff := cmpDiff(m.expectInputCreds, creds); diff != "" {
			return Credentials{}, fmt.Errorf("expect creds match\n%s", diff)
		}
	}
	return m.creds, m.err
}

func TestCredentialsCache_IsCredentialsProvider(t *testing.T) {
	tests := map[string]struct {
		provider CredentialsProvider
		target   CredentialsProvider
		want     bool
	}{
		"nil provider and target": {
			provider: nil,
			target:   nil,
			want:     true,
		},
		"matches value implementations": {
			provider: NewCredentialsCache(AnonymousCredentials{}),
			target:   AnonymousCredentials{},
			want:     true,
		},
		"matches value and pointer implementations, wrapped pointer": {
			provider: NewCredentialsCache(&AnonymousCredentials{}),
			target:   AnonymousCredentials{},
			want:     true,
		},
		"matches value and pointer implementations, pointer target": {
			provider: NewCredentialsCache(AnonymousCredentials{}),
			target:   &AnonymousCredentials{},
			want:     true,
		},
		"does not match mismatched provider types": {
			provider: NewCredentialsCache(AnonymousCredentials{}),
			target:   &stubCredentialsProvider{},
			want:     false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := NewCredentialsCache(tt.provider).IsCredentialsProvider(tt.target); got != tt.want {
				t.Errorf("IsCredentialsProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

var _ isCredentialsProvider = (*CredentialsCache)(nil)

func cmpDiff(e, a interface{}) string {
	if !reflect.DeepEqual(e, a) {
		return fmt.Sprintf("%v != %v", e, a)
	}
	return ""
}
