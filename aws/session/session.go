package session

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/logging"
	log "github.com/sirupsen/logrus"

	"github.com/bpal410/ssm-helpers/aws/config"
)

// NewPool is used to create a pool of AWS sessions with different profile/region permutations.
//
// A profile/region pair whose config cannot be loaded is left out of the pool
// and its error is included in the returned error, so one broken profile does
// not stop the others from being used. Callers should treat an error with an
// empty pool as fatal.
func NewPool(ctx context.Context, profiles []string, regions []string, logger *log.Logger) (*Pool, error) {
	sessions := map[string]*Session{}
	var errs []error

	add := func(name, profile, region string) {
		session, err := newSession(ctx, profile, region, logger)
		if err != nil {
			errs = append(errs, err)
			return
		}
		sessions[name] = session
	}

	for _, region := range regions {
		if stsCredentialsSet() {
			add(fmt.Sprintf("default-%s", region), "", region)
			continue
		}

		for _, profile := range profiles {
			add(fmt.Sprintf("%s-%s", profile, region), profile, region)
		}
	}

	return &Pool{Sessions: sessions}, errors.Join(errs...)
}

func stsCredentialsSet() bool {
	sessionToken := os.Getenv("AWS_SESSION_TOKEN")
	accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	if sessionToken == "" || accessKeyID == "" || secretAccessKey == "" {
		return false
	}

	return true
}

func newSession(ctx context.Context, profile string, region string, logger *log.Logger) (*Session, error) {
	opts := config.NewLoadOptions(profile, region)

	// Route SDK log output through our logger instead of the SDK's default stderr logger
	opts = append(opts, awsconfig.WithLogger(sdkLogger(logger)))

	if logger.IsLevelEnabled(log.TraceLevel) {
		opts = append(opts, awsconfig.WithClientLogMode(
			aws.LogRetries|aws.LogRequestWithBody|aws.LogResponseWithBody,
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("could not load AWS config for profile %q in region %q: %w", profile, region, err)
	}

	return &Session{
		Logger:      logger,
		ProfileName: profile,
		Config:      cfg,
	}, nil
}

// sdkLogger adapts a logrus logger to the SDK's logging interface. SDK warnings
// are logged as warnings; everything else (request/response dumps) at trace.
func sdkLogger(logger *log.Logger) logging.Logger {
	return logging.LoggerFunc(func(classification logging.Classification, format string, v ...interface{}) {
		if classification == logging.Warn {
			logger.Warnf(format, v...)
			return
		}
		logger.Tracef(format, v...)
	})
}
