package config

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"

	"github.com/bpal410/ssm-helpers/util/httpx"
)

const (
	// maxAttempts is the total number of attempts per API call: the first call plus 3 retries.
	maxAttempts = 4

	// maxBackoff caps the delay between retries.
	maxBackoff = 1500 * time.Millisecond
)

// NewLoadOptions returns the options used to load every AWS config in this tool.
//
// An empty profile is left unset, so credentials come from the environment
// (or the SDK's default chain) rather than a named shared config profile. An
// empty region is also left unset, so the profile's or the environment's
// region is used.
func NewLoadOptions(profile string, region string) []func(*awsconfig.LoadOptions) error {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
		awsconfig.WithHTTPClient(httpx.NewDefaultClient()),
		awsconfig.WithRetryer(newRetryer),
		awsconfig.WithAssumeRoleCredentialOptions(func(o *stscreds.AssumeRoleOptions) {
			// Prompt for an MFA code on stdin when a role requires one
			o.TokenProvider = stscreds.StdinTokenProvider
		}),
	}

	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	return opts
}

func newRetryer() aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = maxAttempts
		o.MaxBackoff = maxBackoff

		// The standard retryer limits retries with a client-side token bucket.
		// This tool fans out across many profiles and regions at once, so a burst
		// of throttling would empty the bucket and fail calls that a retry would
		// have saved. The v1 SDK had no such limit.
		o.RateLimiter = ratelimit.None
	})
}
