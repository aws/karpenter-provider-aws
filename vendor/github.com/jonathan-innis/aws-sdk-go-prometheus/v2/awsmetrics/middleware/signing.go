package middleware

import (
	"context"
	"time"

	"github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

func timeSigning(stack *middleware.Stack) error {
	if err := stack.Finalize.Insert(signingStart{}, "Signing", middleware.Before); err != nil {
		return err
	}
	if err := stack.Finalize.Insert(signingEnd{}, "Signing", middleware.After); err != nil {
		return err
	}
	return nil
}

type signingStart struct{}

func (m signingStart) ID() string { return "signingStart" }

func (m signingStart) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, md middleware.Metadata, err error,
) {
	mctx := awsmetrics.Context(ctx)
	attempt, err := mctx.Data().LatestAttempt()
	if err != nil {
		return out, md, err
	}

	attempt.SignStartTime = time.Now().UTC()
	return next.HandleFinalize(ctx, in)
}

type signingEnd struct{}

func (m signingEnd) ID() string { return "signingEnd" }

func (m signingEnd) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, md middleware.Metadata, err error,
) {
	mctx := awsmetrics.Context(ctx)
	attempt, err := mctx.Data().LatestAttempt()
	if err != nil {
		return out, md, err
	}

	attempt.SignEndTime = time.Now().UTC()
	return next.HandleFinalize(ctx, in)
}
