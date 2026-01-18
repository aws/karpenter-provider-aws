package aws

import (
	"context"
	"testing"
)

type anonymousNamedType AnonymousCredentials

func (f anonymousNamedType) Retrieve(ctx context.Context) (Credentials, error) {
	return AnonymousCredentials(f).Retrieve(ctx)
}

func TestIsCredentialsProvider(t *testing.T) {
	tests := map[string]struct {
		provider CredentialsProvider
		target   CredentialsProvider
		want     bool
	}{
		"same implementations": {
			provider: AnonymousCredentials{},
			target:   AnonymousCredentials{},
			want:     true,
		},
		"same implementations, pointer target": {
			provider: AnonymousCredentials{},
			target:   &AnonymousCredentials{},
			want:     true,
		},
		"same implementations, pointer provider": {
			provider: &AnonymousCredentials{},
			target:   AnonymousCredentials{},
			want:     true,
		},
		"same implementations, both pointers": {
			provider: &AnonymousCredentials{},
			target:   &AnonymousCredentials{},
			want:     true,
		},
		"different implementations, nil target": {
			provider: AnonymousCredentials{},
			target:   nil,
			want:     false,
		},
		"different implementations, nil provider": {
			provider: nil,
			target:   AnonymousCredentials{},
			want:     false,
		},
		"different implementations": {
			provider: AnonymousCredentials{},
			target:   &stubCredentialsProvider{},
			want:     false,
		},
		"nil provider and target": {
			provider: nil,
			target:   nil,
			want:     true,
		},
		"implements IsCredentialsProvider, match": {
			provider: NewCredentialsCache(AnonymousCredentials{}),
			target:   AnonymousCredentials{},
			want:     true,
		},
		"implements IsCredentialsProvider, no match": {
			provider: NewCredentialsCache(AnonymousCredentials{}),
			target:   &stubCredentialsProvider{},
			want:     false,
		},
		"named types aliasing underlying types": {
			provider: AnonymousCredentials{},
			target:   anonymousNamedType{},
			want:     false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := IsCredentialsProvider(tt.provider, tt.target); got != tt.want {
				t.Errorf("IsCredentialsProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}
