package aws

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

func BenchmarkCredentialsCache_Retrieve(b *testing.B) {
	provider := CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
		return Credentials{
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
			Source:          "benchmark",
		}, nil
	})

	cases := []int{1, 10, 100, 500, 1000, 10000}
	for _, c := range cases {
		b.Run(strconv.Itoa(c), func(b *testing.B) {
			p := NewCredentialsCache(provider)
			var wg sync.WaitGroup
			wg.Add(c)
			for i := 0; i < c; i++ {
				go func() {
					for j := 0; j < b.N; j++ {
						v, err := p.Retrieve(context.Background())
						if err != nil {
							b.Errorf("expect no error %v, %v", v, err)
						}
					}
					wg.Done()
				}()
			}
			b.ResetTimer()

			wg.Wait()
		})
	}
}

func BenchmarkCredentialsCache_Retrieve_Invalidate(b *testing.B) {
	provider := CredentialsProviderFunc(func(ctx context.Context) (Credentials, error) {
		time.Sleep(time.Millisecond)
		return Credentials{
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
			Source:          "benchmark",
		}, nil
	})

	expRates := []int{10000, 1000, 100}
	cases := []int{1, 10, 100, 500, 1000, 10000}
	for _, expRate := range expRates {
		for _, c := range cases {
			b.Run(fmt.Sprintf("%d-%d", expRate, c), func(b *testing.B) {
				p := NewCredentialsCache(provider)
				var wg sync.WaitGroup
				wg.Add(c)
				for i := 0; i < c; i++ {
					go func(id int) {
						for j := 0; j < b.N; j++ {
							v, err := p.Retrieve(context.Background())
							if err != nil {
								b.Errorf("expect no error %v, %v", v, err)
							}
							// periodically expire creds to cause rwlock
							if id == 0 && j%expRate == 0 {
								p.Invalidate()
							}
						}
						wg.Done()
					}(i)
				}
				b.ResetTimer()

				wg.Wait()
			})
		}
	}
}
