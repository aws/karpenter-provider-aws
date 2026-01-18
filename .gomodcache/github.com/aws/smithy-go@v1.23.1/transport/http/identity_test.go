package http

import (
	"context"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/auth"
)

func TestIdentity(t *testing.T) {
	resolver := auth.AnonymousIdentityResolver{}
	identity, _ := resolver.GetIdentity(context.TODO(), smithy.Properties{})
	if _, ok := identity.(*auth.AnonymousIdentity); !ok {
		t.Errorf("Anonymous identity resolver does not produce correct identity, expected it to be of type auth.AnonymousIdentity")
	}
}