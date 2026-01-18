package defaults

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestConfigV1(t *testing.T) {
	cases := []struct {
		Mode     aws.DefaultsMode
		Expected Configuration
	}{
		{
			Mode: aws.DefaultsModeStandard,
			Expected: Configuration{
				ConnectTimeout:        aws.Duration(2000 * time.Millisecond),
				TLSNegotiationTimeout: aws.Duration(2000 * time.Millisecond),
				RetryMode:             aws.RetryModeStandard,
			},
		},
		{
			Mode: aws.DefaultsModeInRegion,
			Expected: Configuration{
				ConnectTimeout:        aws.Duration(1000 * time.Millisecond),
				TLSNegotiationTimeout: aws.Duration(1000 * time.Millisecond),
				RetryMode:             aws.RetryModeStandard,
			},
		},
		{
			Mode: aws.DefaultsModeCrossRegion,
			Expected: Configuration{
				ConnectTimeout:        aws.Duration(2800 * time.Millisecond),
				TLSNegotiationTimeout: aws.Duration(2800 * time.Millisecond),
				RetryMode:             aws.RetryModeStandard,
			},
		},
		{
			Mode: aws.DefaultsModeMobile,
			Expected: Configuration{
				ConnectTimeout:        aws.Duration(10000 * time.Millisecond),
				TLSNegotiationTimeout: aws.Duration(11000 * time.Millisecond),
				RetryMode:             aws.RetryModeAdaptive,
			},
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got, err := v1TestResolver(tt.Mode)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			if diff := cmpDiff(tt.Expected, got); len(diff) > 0 {
				t.Error(diff)
			}
		})
	}
}

func cmpDiff(e, a interface{}) string {
	if !reflect.DeepEqual(e, a) {
		return fmt.Sprintf("%v != %v", e, a)
	}
	return ""
}
