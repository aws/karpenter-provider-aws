package middleware

import (
	"context"
	"time"

	"github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

func timeGetIdentity(stack *middleware.Stack) error {
	if err := stack.Finalize.Insert(getIdentityStart{}, "GetIdentity", middleware.Before); err != nil {
		return err
	}
	if err := stack.Finalize.Insert(getIdentityEnd{}, "GetIdentity", middleware.After); err != nil {
		return err
	}
	return nil
}

type getIdentityStart struct{}

func (m getIdentityStart) ID() string { return "getIdentityStart" }

func (m getIdentityStart) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, md middleware.Metadata, err error,
) {
	mctx := awsmetrics.Context(ctx)
	mctx.Data().GetIdentityStartTime = time.Now().UTC()
	return next.HandleFinalize(ctx, in)
}

type getIdentityEnd struct{}

func (m getIdentityEnd) ID() string { return "getIdentityEnd" }

func (m getIdentityEnd) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, md middleware.Metadata, err error,
) {
	mctx := awsmetrics.Context(ctx)
	mctx.Data().GetIdentityEndTime = time.Now().UTC()
	return next.HandleFinalize(ctx, in)
}
