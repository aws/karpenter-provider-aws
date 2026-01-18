package endpoints_test

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/internal/endpoints/v2"
	"github.com/aws/smithy-go/logging"
)

type testCase struct {
	Region   string
	Options  endpoints.Options
	Expected aws.Endpoint
	WantErr  bool
}

type serviceTest struct {
	Partitions endpoints.Partitions
	Cases      []testCase
}

var partitionRegexp = struct {
	Aws    *regexp.Regexp
	AwsIso *regexp.Regexp
}{

	Aws:    regexp.MustCompile("^(us|eu|ap|sa|ca|me|af)\\-\\w+\\-\\d+$"),
	AwsIso: regexp.MustCompile("^us\\-iso\\-\\w+\\-\\d+$"),
}

var testCases = map[string]serviceTest{
	"default-pattern-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "default-pattern-service.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "default-pattern-service-fips.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "default-pattern-service-fips.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "default-pattern-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.Aws,
				IsRegionalized: true,
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "af-south-1",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region: "us-west-2",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname: "default-pattern-service-fips.us-west-2.amazonaws.com",
					},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname: "default-pattern-service.us-west-2.api.aws",
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "default-pattern-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service-fips.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service-fips.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service.us-west-2.api.aws",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://default-pattern-service.af-south-1.api.aws",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
		},
	},
	"global-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "global-service.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "global-service-fips.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "global-service-fips.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "global-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:       partitionRegexp.Aws,
				IsRegionalized:    false,
				PartitionEndpoint: "aws-global",
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "aws-global",
					}: endpoints.Endpoint{
						Hostname: "global-service.amazonaws.com",
						CredentialScope: endpoints.CredentialScope{
							Region: "us-east-1",
						},
					},
					endpoints.EndpointKey{
						Region:  "aws-global",
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname: "global-service-fips.amazonaws.com",
						CredentialScope: endpoints.CredentialScope{
							Region: "us-east-1",
						},
					},
					endpoints.EndpointKey{
						Region:  "aws-global",
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname: "global-service.api.aws",
						CredentialScope: endpoints.CredentialScope{
							Region: "us-east-1",
						},
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "global-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "aws-global",
				Expected: aws.Endpoint{
					URL:           "https://global-service.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "aws-global",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://global-service-fips.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "foo",
				Expected: aws.Endpoint{
					URL:           "https://global-service.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "foo",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://global-service-fips.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "aws-global",
				Expected: aws.Endpoint{
					URL:           "https://global-service.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "aws-global",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://global-service.api.aws",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "foo",
				Expected: aws.Endpoint{
					URL:           "https://global-service.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "foo",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://global-service.api.aws",
					PartitionID:   "aws",
					SigningRegion: "us-east-1",
					SigningMethod: "v4",
				},
			},
		},
	},
	"multi-variant-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "multi-variant-service.dualstack.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "fips.multi-variant-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "fips.multi-variant-service.dualstack.{region}.new.dns.suffix",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "multi-variant-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.Aws,
				IsRegionalized: true,
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "af-south-1",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region: "us-west-2",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname: "fips.multi-variant-service.dualstack.us-west-2.new.dns.suffix",
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "multi-variant-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://multi-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://multi-variant-service.dualstack.us-west-2.api.aws",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.multi-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint:      aws.FIPSEndpointStateEnabled,
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.multi-variant-service.dualstack.us-west-2.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://multi-variant-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://multi-variant-service.dualstack.af-south-1.api.aws",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.multi-variant-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint:      aws.FIPSEndpointStateEnabled,
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.multi-variant-service.dualstack.af-south-1.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
		},
	},
	"override-endpoint-variant-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "override-endpoint-variant-service.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "override-endpoint-variant-service-fips.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "override-endpoint-variant-service-fips.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "override-endpoint-variant-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.Aws,
				IsRegionalized: true,
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "af-south-1",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region: "us-west-2",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname: "fips.override-endpoint-variant-service.us-west-2.amazonaws.com",
					},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname: "override-endpoint-variant-service.dualstack.us-west-2.amazonaws.com",
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "override-endpoint-variant-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.override-endpoint-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service-fips.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service.dualstack.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-endpoint-variant-service.af-south-1.api.aws",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
		},
	},
	"override-variant-dns-suffix-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "override-variant-dns-suffix-service.{region}.new.dns.suffix",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "override-variant-dns-suffix-service-fips.{region}.new.dns.suffix",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "override-variant-dns-suffix-service-fips.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "override-variant-dns-suffix-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.Aws,
				IsRegionalized: true,
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "af-south-1",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region: "us-west-2",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname: "override-variant-dns-suffix-service-fips.us-west-2.new.dns.suffix",
					},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname: "override-variant-dns-suffix-service.us-west-2.new.dns.suffix",
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "override-variant-dns-suffix-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service-fips.us-west-2.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service-fips.af-south-1.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service.us-west-2.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-dns-suffix-service.af-south-1.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
		},
	},
	"override-variant-hostname-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "override-variant-hostname-service.dualstack.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "fips.override-variant-hostname-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "override-variant-hostname-service-fips.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "override-variant-hostname-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.Aws,
				IsRegionalized: true,
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "af-south-1",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region: "us-west-2",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname: "fips.override-variant-hostname-service.us-west-2.amazonaws.com",
					},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname: "override-variant-hostname-service.dualstack.us-west-2.api.aws",
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "override-variant-hostname-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-hostname-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.override-variant-hostname-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-hostname-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.override-variant-hostname-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-hostname-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-hostname-service.dualstack.us-west-2.api.aws",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-hostname-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-hostname-service.dualstack.af-south-1.api.aws",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
		},
	},
	"override-variant-service": {
		Partitions: endpoints.Partitions{
			{
				ID: "aws",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname:          "override-variant-service.dualstack.{region}.new.dns.suffix",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname:          "fips.override-variant-service.{region}.new.dns.suffix",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: endpoints.FIPSVariant | endpoints.DualStackVariant,
					}: {
						Hostname:          "override-variant-service-fips.{region}.api.aws",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
					{
						Variant: 0,
					}: {
						Hostname:          "override-variant-service.{region}.amazonaws.com",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.Aws,
				IsRegionalized: true,
				Endpoints: endpoints.Endpoints{
					endpoints.EndpointKey{
						Region: "af-south-1",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region: "us-west-2",
					}: endpoints.Endpoint{},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.FIPSVariant,
					}: {
						Hostname: "fips.override-variant-service.us-west-2.new.dns.suffix",
					},
					endpoints.EndpointKey{
						Region:  "us-west-2",
						Variant: endpoints.DualStackVariant,
					}: {
						Hostname: "override-variant-service.dualstack.us-west-2.new.dns.suffix",
					},
				},
			},
			{
				ID: "aws-iso",
				Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
					{
						Variant: 0,
					}: {
						Hostname:          "override-variant-service.{region}.c2s.ic.gov",
						Protocols:         []string{"https"},
						SignatureVersions: []string{"v4"},
					},
				},
				RegionRegex:    partitionRegexp.AwsIso,
				IsRegionalized: true,
			},
		},
		Cases: []testCase{
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.override-variant-service.us-west-2.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://fips.override-variant-service.af-south-1.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-service.us-west-2.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "us-west-2",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-service.dualstack.us-west-2.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "us-west-2",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Expected: aws.Endpoint{
					URL:           "https://override-variant-service.af-south-1.amazonaws.com",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
			{
				Region: "af-south-1",
				Options: endpoints.Options{
					UseDualStackEndpoint: aws.DualStackEndpointStateEnabled,
				},
				Expected: aws.Endpoint{
					URL:           "https://override-variant-service.dualstack.af-south-1.new.dns.suffix",
					PartitionID:   "aws",
					SigningRegion: "af-south-1",
					SigningMethod: "v4",
				},
			},
		},
	},
}

func TestResolveEndpoint(t *testing.T) {
	for service := range testCases {
		t.Run(service, func(t *testing.T) {
			partitions := testCases[service].Partitions
			testCases := testCases[service].Cases

			for i, tt := range testCases {
				t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
					endpoint, err := partitions.ResolveEndpoint(tt.Region, tt.Options)
					if (err != nil) != (tt.WantErr) {
						t.Errorf("WantErr=%v, got error: %v", tt.WantErr, err)
					}
					if diff := cmpDiff(tt.Expected, endpoint); len(diff) > 0 {
						t.Error(diff)
					}
				})
			}
		})
	}
}

func TestDisableHTTPS(t *testing.T) {
	partitions := endpoints.Partitions{
		endpoints.Partition{
			ID:          "aws",
			RegionRegex: partitionRegexp.Aws,
			Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
				{}: {
					Hostname:  "foo.bar.tld",
					Protocols: []string{"https", "http"},
				},
			},
		},
	}

	endpoint, err := partitions.ResolveEndpoint("us-west-2", endpoints.Options{})
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	if e, a := "https://foo.bar.tld", endpoint.URL; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}

	endpoint, err = partitions.ResolveEndpoint("us-west-2", endpoints.Options{DisableHTTPS: true})
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	if e, a := "http://foo.bar.tld", endpoint.URL; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func TestLogDeprecated(t *testing.T) {
	partitions := endpoints.Partitions{
		endpoints.Partition{
			ID:          "aws",
			RegionRegex: partitionRegexp.Aws,
			Defaults: map[endpoints.DefaultKey]endpoints.Endpoint{
				{}: {
					Hostname:  "foo.{region}.bar.tld",
					Protocols: []string{"https", "http"},
				},
				{
					Variant: endpoints.FIPSVariant,
				}: {
					Hostname: "foo-fips.{region}.bar.tld",
				},
			},
			Endpoints: map[endpoints.EndpointKey]endpoints.Endpoint{
				{
					Region: "foo",
				}: {},
				{
					Region: "bar",
				}: {
					Deprecated: aws.TrueTernary,
				},
				{
					Region:  "bar",
					Variant: endpoints.FIPSVariant,
				}: {
					Deprecated: aws.TrueTernary,
				},
			},
		},
	}

	cases := []struct {
		Region      string
		Options     endpoints.Options
		Expected    aws.Endpoint
		SetupLogger func() (logging.Logger, func(*testing.T))
		WantErr     bool
	}{
		{
			Region: "foo",
			Expected: aws.Endpoint{
				URL:           "https://foo.foo.bar.tld",
				PartitionID:   "aws",
				SigningRegion: "foo",
				SigningMethod: "v4",
			},
		},
		{
			Region: "bar",
			Options: endpoints.Options{
				LogDeprecated: true,
			},
			Expected: aws.Endpoint{
				URL:           "https://foo.bar.bar.tld",
				PartitionID:   "aws",
				SigningRegion: "bar",
				SigningMethod: "v4",
			},
		},
		{
			Region: "bar",
			Options: endpoints.Options{
				LogDeprecated:   true,
				UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
			},
			Expected: aws.Endpoint{
				URL:           "https://foo-fips.bar.bar.tld",
				PartitionID:   "aws",
				SigningRegion: "bar",
				SigningMethod: "v4",
			},
		},
		{
			Region: "bar",
			Options: endpoints.Options{
				LogDeprecated: true,
			},
			SetupLogger: func() (logging.Logger, func(*testing.T)) {
				buffer := bytes.NewBuffer(nil)
				return &logging.StandardLogger{
						Logger: log.New(buffer, "", 0),
					}, func(t *testing.T) {
						if diff := cmpDiff("WARN endpoint identifier \"bar\", url \"https://foo.bar.bar.tld\" marked as deprecated\n", buffer.String()); len(diff) > 0 {
							t.Error(diff)
						}
					}
			},
			Expected: aws.Endpoint{
				URL:           "https://foo.bar.bar.tld",
				PartitionID:   "aws",
				SigningRegion: "bar",
				SigningMethod: "v4",
			},
		},
		{
			Region: "bar",
			Options: endpoints.Options{
				LogDeprecated:   true,
				UseFIPSEndpoint: aws.FIPSEndpointStateEnabled,
			},
			SetupLogger: func() (logging.Logger, func(*testing.T)) {
				buffer := bytes.NewBuffer(nil)
				return &logging.StandardLogger{
						Logger: log.New(buffer, "", 0),
					}, func(t *testing.T) {
						if diff := cmpDiff("WARN endpoint identifier \"bar\", url \"https://foo-fips.bar.bar.tld\" marked as deprecated\n", buffer.String()); len(diff) > 0 {
							t.Error(diff)
						}
					}
			},
			Expected: aws.Endpoint{
				URL:           "https://foo-fips.bar.bar.tld",
				PartitionID:   "aws",
				SigningRegion: "bar",
				SigningMethod: "v4",
			},
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var verifyLog func(*testing.T)
			if tt.SetupLogger != nil {
				tt.Options.Logger, verifyLog = tt.SetupLogger()
			}

			endpoint, err := partitions.ResolveEndpoint(tt.Region, tt.Options)
			if (err != nil) != tt.WantErr {
				t.Errorf("WantErr(%v), got error %v", tt.WantErr, err)
			}

			if diff := cmpDiff(tt.Expected, endpoint); len(diff) > 0 {
				t.Error(diff)
			}

			if verifyLog != nil {
				verifyLog(t)
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
