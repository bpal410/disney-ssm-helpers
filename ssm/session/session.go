package session

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
)

// ConnectionStatusAPI is the part of the SSM API used to check session readiness
type ConnectionStatusAPI interface {
	GetConnectionStatus(ctx context.Context, params *ssm.GetConnectionStatusInput, optFns ...func(*ssm.Options)) (*ssm.GetConnectionStatusOutput, error)
}

// CheckSessionReadiness takes an SSM API client and instance ID and verifies whether or not the instance is available for start-session functionality
func CheckSessionReadiness(ctx context.Context, client ConnectionStatusAPI, instanceID string) (connected bool, err error) {
	// Call GetConnectionStatus to determine if the given instance is ready for a session
	output, err := client.GetConnectionStatus(ctx, &ssm.GetConnectionStatusInput{
		Target: aws.String(instanceID),
	})
	if err != nil {
		// Suppress errors due to rate limiting; the retryer has already backed off
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "ThrottlingException" {
			return false, nil
		}
		return false, err
	}

	// The API returns "connected"/"notconnected", while older SDK models (and some
	// docs) capitalise them, so compare without regard to case
	return strings.EqualFold(string(output.Status), string(ssmtypes.ConnectionStatusConnected)), nil
}
