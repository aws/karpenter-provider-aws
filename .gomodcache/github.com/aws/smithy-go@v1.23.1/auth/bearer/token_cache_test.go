package bearer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var _ TokenProvider = (*TokenCache)(nil)

func TestTokenCache_cache(t *testing.T) {
	expectToken := Token{
		Value: "abc123",
	}

	var retrieveCalled bool
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		if retrieveCalled {
			t.Fatalf("expect wrapped provider to be called once")
		}
		retrieveCalled = true
		return expectToken, nil
	}))

	token, err := provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if expectToken != token {
		t.Errorf("expect token match: %v != %v", expectToken, token)
	}

	for i := 0; i < 100; i++ {
		token, err := provider.RetrieveBearerToken(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
		if expectToken != token {
			t.Errorf("expect token match: %v != %v", expectToken, token)
		}
	}
}

func TestTokenCache_cacheConcurrent(t *testing.T) {
	expectToken := Token{
		Value: "abc123",
	}

	var retrieveCalled bool
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		if retrieveCalled {
			t.Fatalf("expect wrapped provider to be called once")
		}
		retrieveCalled = true
		return expectToken, nil
	}))

	token, err := provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if expectToken != token {
		t.Errorf("expect token match: %v != %v", expectToken, token)
	}

	for i := 0; i < 100; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			token, err := provider.RetrieveBearerToken(context.Background())
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			if expectToken != token {
				t.Errorf("expect token match: %v != %v", expectToken, token)
			}
		})
	}
}

func TestTokenCache_expired(t *testing.T) {
	origTimeNow := timeNow
	defer func() { timeNow = origTimeNow }()

	timeNow = func() time.Time { return time.Time{} }

	expectToken := Token{
		Value:     "abc123",
		CanExpire: true,
		Expires:   timeNow().Add(10 * time.Minute),
	}
	refreshedToken := Token{
		Value:     "refreshed-abc123",
		CanExpire: true,
		Expires:   timeNow().Add(30 * time.Minute),
	}

	retrievedCount := new(int32)
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		if atomic.AddInt32(retrievedCount, 1) > 1 {
			return refreshedToken, nil
		}
		return expectToken, nil
	}))

	for i := 0; i < 10; i++ {
		token, err := provider.RetrieveBearerToken(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
		if expectToken != token {
			t.Errorf("expect token match: %v != %v", expectToken, token)
		}
	}
	if e, a := 1, int(atomic.LoadInt32(retrievedCount)); e != a {
		t.Errorf("expect %v provider calls, got %v", e, a)
	}

	// Offset time for refresh
	timeNow = func() time.Time {
		return (time.Time{}).Add(10 * time.Minute)
	}

	token, err := provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if refreshedToken != token {
		t.Errorf("expect refreshed token match: %v != %v", refreshedToken, token)
	}
	if e, a := 2, int(atomic.LoadInt32(retrievedCount)); e != a {
		t.Errorf("expect %v provider calls, got %v", e, a)
	}
}

func TestTokenCache_cancelled(t *testing.T) {
	providerRunning := make(chan struct{})
	providerDone := make(chan struct{})
	var onceClose sync.Once
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		onceClose.Do(func() { close(providerRunning) })

		// Provider running never receives context cancel so that if the first
		// retrieve call is canceled all subsequent retrieve callers won't get
		// canceled as well.
		select {
		case <-providerDone:
			return Token{Value: "abc123"}, nil
		case <-ctx.Done():
			return Token{}, fmt.Errorf("unexpected context canceled, %w", ctx.Err())
		}
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Retrieve that will have its context canceled, should return error, but
	// underlying provider retrieve will continue to block in the background.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		_, err := provider.RetrieveBearerToken(ctx)
		if err == nil {
			t.Errorf("expect error, got none")

		} else if e, a := "unexpected context canceled", err.Error(); strings.Contains(a, e) {
			t.Errorf("unexpected context canceled received, %v", err)

		} else if e, a := "context canceled", err.Error(); !strings.Contains(a, e) {
			t.Errorf("expect %v error in, %v", e, a)
		}
	}()

	<-providerRunning

	// Retrieve that will be added to existing single flight group, (or create
	// a new group). Returning valid token.
	wg.Add(1)
	go func() {
		defer wg.Done()

		token, err := provider.RetrieveBearerToken(context.Background())
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		} else {
			expect := Token{Value: "abc123"}
			if expect != token {
				t.Errorf("expect token retrieve match: %v != %v", expect, token)
			}
		}
	}()
	close(providerDone)

	wg.Wait()
}

func TestTokenCache_cancelledWithTimeout(t *testing.T) {
	providerReady := make(chan struct{})
	var providerReadCloseOnce sync.Once
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		providerReadCloseOnce.Do(func() { close(providerReady) })

		<-ctx.Done()
		return Token{}, fmt.Errorf("token retrieve timeout, %w", ctx.Err())
	}), func(o *TokenCacheOptions) {
		o.RetrieveBearerTokenTimeout = time.Millisecond
	})

	var wg sync.WaitGroup

	// Spin up additional retrieves that will be deduplicated and block on the
	// original retrieve call.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-providerReady

			_, err := provider.RetrieveBearerToken(context.Background())
			if err == nil {
				t.Errorf("expect error, got none")

			} else if e, a := "token retrieve timeout", err.Error(); !strings.Contains(a, e) {
				t.Errorf("expect %v error in, %v", e, a)
			}
		}()
	}

	_, err := provider.RetrieveBearerToken(context.Background())
	if err == nil {
		t.Errorf("expect error, got none")

	} else if e, a := "token retrieve timeout", err.Error(); !strings.Contains(a, e) {
		t.Errorf("expect %v error in, %v", e, a)
	}

	wg.Wait()
}

func TestTokenCache_asyncRefresh(t *testing.T) {
	origTimeNow := timeNow
	defer func() { timeNow = origTimeNow }()

	timeNow = func() time.Time { return time.Time{} }

	expectToken := Token{
		Value:     "abc123",
		CanExpire: true,
		Expires:   timeNow().Add(10 * time.Minute),
	}
	refreshedToken := Token{
		Value:     "refreshed-abc123",
		CanExpire: true,
		Expires:   timeNow().Add(30 * time.Minute),
	}

	retrievedCount := new(int32)
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		c := atomic.AddInt32(retrievedCount, 1)
		switch {
		case c == 1:
			return expectToken, nil
		case c > 1 && c < 5:
			return Token{}, fmt.Errorf("some error")
		case c == 5:
			return refreshedToken, nil
		default:
			return Token{}, fmt.Errorf("unexpected error")
		}
	}), func(o *TokenCacheOptions) {
		o.RefreshBeforeExpires = 5 * time.Minute
	})

	// 1: Initial retrieve to cache token
	token, err := provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if expectToken != token {
		t.Errorf("expect token match: %v != %v", expectToken, token)
	}

	// 2-5: Offset time for subsequent calls to retrieve to trigger asynchronous
	// refreshes.
	timeNow = func() time.Time {
		return (time.Time{}).Add(6 * time.Minute)
	}

	for i := 0; i < 4; i++ {
		token, err := provider.RetrieveBearerToken(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
		if expectToken != token {
			t.Errorf("expect token match: %v != %v", expectToken, token)
		}
	}
	// Wait for all async refreshes to complete
	testWaitAsyncRefreshDone(provider)

	if c := int(atomic.LoadInt32(retrievedCount)); c < 2 || c > 5 {
		t.Fatalf("expect async refresh to be called [2,5) times, got, %v", c)
	}

	// Ensure enough retrieves have been done to trigger refresh.
	if c := atomic.LoadInt32(retrievedCount); c != 5 {
		atomic.StoreInt32(retrievedCount, 4)
		token, err := provider.RetrieveBearerToken(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
		if expectToken != token {
			t.Errorf("expect token match: %v != %v", expectToken, token)
		}
		testWaitAsyncRefreshDone(provider)
	}

	// Last async refresh will succeed and update cached token, expect the next
	// call to get refreshed token.
	token, err = provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if refreshedToken != token {
		t.Errorf("expect refreshed token match: %v != %v", refreshedToken, token)
	}
}

func TestTokenCache_asyncRefreshWithMinDelay(t *testing.T) {
	origTimeNow := timeNow
	defer func() { timeNow = origTimeNow }()

	timeNow = func() time.Time { return time.Time{} }

	expectToken := Token{
		Value:     "abc123",
		CanExpire: true,
		Expires:   timeNow().Add(10 * time.Minute),
	}
	refreshedToken := Token{
		Value:     "refreshed-abc123",
		CanExpire: true,
		Expires:   timeNow().Add(30 * time.Minute),
	}

	retrievedCount := new(int32)
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		c := atomic.AddInt32(retrievedCount, 1)
		switch {
		case c == 1:
			return expectToken, nil
		case c > 1 && c < 5:
			return Token{}, fmt.Errorf("some error")
		case c == 5:
			return refreshedToken, nil
		default:
			return Token{}, fmt.Errorf("unexpected error")
		}
	}), func(o *TokenCacheOptions) {
		o.RefreshBeforeExpires = 5 * time.Minute
		o.AsyncRefreshMinimumDelay = 30 * time.Second
	})

	// 1: Initial retrieve to cache token
	token, err := provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if expectToken != token {
		t.Errorf("expect token match: %v != %v", expectToken, token)
	}

	// 2-5: Offset time for subsequent calls to retrieve to trigger asynchronous
	// refreshes.
	timeNow = func() time.Time {
		return (time.Time{}).Add(6 * time.Minute)
	}

	for i := 0; i < 4; i++ {
		token, err := provider.RetrieveBearerToken(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
		if expectToken != token {
			t.Errorf("expect token match: %v != %v", expectToken, token)
		}
		// Wait for all async refreshes to complete ensure not deduped
		testWaitAsyncRefreshDone(provider)
	}

	// Only a single refresh attempt is expected.
	if e, a := 2, int(atomic.LoadInt32(retrievedCount)); e != a {
		t.Fatalf("expect %v min async refresh, got %v", e, a)
	}

	// Move time forward to ensure another async refresh is triggered.
	timeNow = func() time.Time { return (time.Time{}).Add(7 * time.Minute) }
	// Make sure the next attempt refreshes the token
	atomic.StoreInt32(retrievedCount, 4)

	// Do async retrieve that will succeed refreshing in background.
	token, err = provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if expectToken != token {
		t.Errorf("expect token match: %v != %v", expectToken, token)
	}
	// Wait for all async refreshes to complete ensure not deduped
	testWaitAsyncRefreshDone(provider)

	// Last async refresh will succeed and update cached token, expect the next
	// call to get refreshed token.
	token, err = provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if refreshedToken != token {
		t.Errorf("expect refreshed token match: %v != %v", refreshedToken, token)
	}
}

func TestTokenCache_disableAsyncRefresh(t *testing.T) {
	origTimeNow := timeNow
	defer func() { timeNow = origTimeNow }()

	timeNow = func() time.Time { return time.Time{} }

	expectToken := Token{
		Value:     "abc123",
		CanExpire: true,
		Expires:   timeNow().Add(10 * time.Minute),
	}
	refreshedToken := Token{
		Value:     "refreshed-abc123",
		CanExpire: true,
		Expires:   timeNow().Add(30 * time.Minute),
	}

	retrievedCount := new(int32)
	provider := NewTokenCache(TokenProviderFunc(func(ctx context.Context) (Token, error) {
		c := atomic.AddInt32(retrievedCount, 1)
		switch {
		case c == 1:
			return expectToken, nil
		case c > 1 && c < 5:
			return Token{}, fmt.Errorf("some error")
		case c == 5:
			return refreshedToken, nil
		default:
			return Token{}, fmt.Errorf("unexpected error")
		}
	}), func(o *TokenCacheOptions) {
		o.RefreshBeforeExpires = 5 * time.Minute
		o.DisableAsyncRefresh = true
	})

	// 1: Initial retrieve to cache token
	token, err := provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if expectToken != token {
		t.Errorf("expect token match: %v != %v", expectToken, token)
	}

	// Update time into refresh window before token expires
	timeNow = func() time.Time {
		return (time.Time{}).Add(6 * time.Minute)
	}

	for i := 0; i < 3; i++ {
		_, err = provider.RetrieveBearerToken(context.Background())
		if err == nil {
			t.Fatalf("expect error, got none")
		}
		if e, a := "some error", err.Error(); !strings.Contains(a, e) {
			t.Fatalf("expect %v error in %v", e, a)
		}
		if e, a := i+2, int(atomic.LoadInt32(retrievedCount)); e != a {
			t.Fatalf("expect %v retrieveCount, got %v", e, a)
		}
	}
	if e, a := 4, int(atomic.LoadInt32(retrievedCount)); e != a {
		t.Fatalf("expect %v retrieveCount, got %v", e, a)
	}

	// Last refresh will succeed and update cached token, expect the next
	// call to get refreshed token.
	token, err = provider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	if refreshedToken != token {
		t.Errorf("expect refreshed token match: %v != %v", refreshedToken, token)
	}
}

func testWaitAsyncRefreshDone(provider *TokenCache) {
	asyncResCh := provider.sfGroup.DoChan("async-refresh", func() (interface{}, error) {
		return nil, nil
	})
	<-asyncResCh
}
