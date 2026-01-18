package http

import (
	"testing"
	"github.com/aws/smithy-go/auth"
)

func TestAnonymousScheme(t *testing.T) {
	expectedID := auth.SchemeIDAnonymous
	scheme := NewAnonymousScheme()
	actualID := scheme.SchemeID()
	if expectedID != actualID {
		t.Errorf("AnonymousScheme constructor is not producing the correct scheme ID")
	}

	var expectedSigner Signer = &nopSigner{}
	actualSigner := scheme.Signer()
	if expectedSigner != actualSigner {
		t.Errorf("AnonymousScheme constructor is not producing the correct signer")
	}
}

