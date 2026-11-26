package session

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolateAWSEnv points the SDK at the repo's mock profiles and clears any
// credentials or profile settings from the developer's environment, so these
// tests never read the real ~/.aws.
func isolateAWSEnv(t *testing.T) {
	t.Helper()

	configFile, err := filepath.Abs("../../testing/mock_aws_profiles")
	require.NoError(t, err)

	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	for _, v := range []string{
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
	} {
		t.Setenv(v, "")
	}
}

func discardLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

func TestCreateAWSSession(t *testing.T) {
	isolateAWSEnv(t)
	ctx := context.Background()
	logger := discardLogger()

	t.Run("region from environment when none is given", func(t *testing.T) {
		// Use a made-up region to ensure no overlap with real regions
		t.Setenv("AWS_REGION", "us-test-1")

		session, err := newSession(ctx, "", "", logger)
		require.NoError(t, err)
		assert.Equal(t, "us-test-1", session.Region())
	})

	t.Run("explicit region overrides environment", func(t *testing.T) {
		t.Setenv("AWS_REGION", "us-test-1")

		session, err := newSession(ctx, "", "us-test-2", logger)
		require.NoError(t, err)
		assert.Equal(t, "us-test-2", session.Region())
	})

	t.Run("region from the named profile", func(t *testing.T) {
		session, err := newSession(ctx, "test-account-1", "", logger)
		require.NoError(t, err)
		assert.Equal(t, "us-east-1", session.Region())
		assert.Equal(t, "test-account-1", session.ProfileName)
	})

	t.Run("missing profile is an error", func(t *testing.T) {
		_, err := newSession(ctx, "does-not-exist", "us-east-1", logger)
		assert.Error(t, err)
	})
}

func TestCreateAWSSessionPool(t *testing.T) {
	isolateAWSEnv(t)
	ctx := context.Background()
	logger := discardLogger()

	profiles := []string{"test-account-1", "test-account-2", "test-account-3"}

	t.Run("test session permutations", func(t *testing.T) {
		// Ensure that AWS sessions are created for each permutation of region + profile
		regions := []string{"us-test-1", "us-test-2", "us-test-3"}
		pool, err := NewPool(ctx, profiles, regions, logger)
		require.NoError(t, err)
		assert.Len(t, pool.Sessions, 9)

		// Requires a region to be provided
		pool, err = NewPool(ctx, profiles, nil, logger)
		require.NoError(t, err)
		assert.Len(t, pool.Sessions, 0)
	})

	t.Run("test session deduplication", func(t *testing.T) {
		// Because we're using a map to store all the sessions, duplicate sessions should be filtered out
		regions := []string{"us-test-1", "us-test-2", "us-test-2"}
		dupProfiles := []string{"test-account-1", "test-account-2", "test-account-2"}
		pool, err := NewPool(ctx, dupProfiles, regions, logger)
		require.NoError(t, err)
		assert.Len(t, pool.Sessions, 4)
	})

	t.Run("a bad profile does not stop the others", func(t *testing.T) {
		pool, err := NewPool(ctx, []string{"test-account-1", "does-not-exist"}, []string{"us-test-1"}, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does-not-exist")
		assert.Len(t, pool.Sessions, 1)
		assert.Contains(t, pool.Sessions, "test-account-1-us-test-1")
	})

	t.Run("environment credentials use a single default session per region", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
		t.Setenv("AWS_SESSION_TOKEN", "token")

		pool, err := NewPool(ctx, profiles, []string{"us-test-1", "us-test-2"}, logger)
		require.NoError(t, err)
		assert.Len(t, pool.Sessions, 2)
		assert.Contains(t, pool.Sessions, "default-us-test-1")
	})
}
