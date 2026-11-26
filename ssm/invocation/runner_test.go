package invocation

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

	mocks "github.com/bpal410/ssm-helpers/testing"
)

func TestGetResult(t *testing.T) {
	ctx := context.Background()
	mockSvc := &mocks.MockSSMClient{}

	t.Run("valid ID", func(t *testing.T) {
		result, err := GetResult(ctx, mockSvc, "success-id", "i-123")
		assert.NoError(t, err)
		assert.Equal(t, "success-id", aws.ToString(result.CommandId))
		assert.Equal(t, "i-123", aws.ToString(result.InstanceId))
	})

	t.Run("invalid ID", func(t *testing.T) {
		result, err := GetResult(ctx, mockSvc, "bad-id", "i-123")
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "InvalidCommandId")
	})
}
