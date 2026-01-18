package rand_test

import (
	"bytes"
	mathrand "math/rand"
	"testing"
	"time"

	"github.com/aws/smithy-go/rand"
)

func TestUUID(t *testing.T) {
	randSrc := make([]byte, 32)
	for i := 16; i < len(randSrc); i++ {
		randSrc[i] = 1
	}

	uuid := rand.NewUUID(bytes.NewReader(randSrc))

	v, err := uuid.GetUUID()
	if err != nil {
		t.Fatalf("expect no error getting zero UUID, got %v", err)
	}
	if e, a := `00000000-0000-4000-8000-000000000000`, v; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}

	v, err = uuid.GetUUID()
	if err != nil {
		t.Fatalf("expect no error getting ones UUID, got %v", err)
	}
	if e, a := `01010101-0101-4101-8101-010101010101`, v; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func BenchmarkUUID_GetUUID(b *testing.B) {
	src := mathrand.NewSource(time.Now().Unix())
	uuid := rand.NewUUID(mathrand.New(src))

	for i := 0; i < b.N; i++ {
		_, err := uuid.GetUUID()
		if err != nil {
			b.Fatal(err)
		}
	}
}
