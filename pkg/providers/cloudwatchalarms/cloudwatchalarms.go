package cloudwatchalarms

import (
	"context"
	"sync"

	cloudwatchTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Provider interface {
	List(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]cloudwatchTypes.MetricAlarm, error)
}

type DefaultProvider struct {
	sync.Mutex
	cloudwatchapi sdk.CloudWatchAPI
	cache         *cache.Cache
	cm            *pretty.ChangeMonitor
}

func NewDefaultProvider(cloudwatchapi sdk.CloudWatchAPI, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		cloudwatchapi: cloudwatchapi,
		cm:            pretty.NewChangeMonitor(),
	}
}
