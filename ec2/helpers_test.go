package ec2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

	mocks "github.com/bpal410/ssm-helpers/testing"
)

func TestGetEC2InstanceInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("get instance info from IDs", func(t *testing.T) {
		mockSvc := &mocks.MockEC2Client{}
		info, err := GetEC2InstanceInfo(ctx, mockSvc, []string{"i-123", "i-456", "i-789"})

		assert.NoError(t, err)
		assert.Len(t, info, 3)
		assert.Equal(t, "vpc-456", aws.ToString(info["i-456"].VpcId))
	})

	t.Run("instances are keyed by ID, not position", func(t *testing.T) {
		mockSvc := &mocks.MockEC2Client{}
		info, err := GetEC2InstanceInfo(ctx, mockSvc, []string{"i-789", "i-123"})

		assert.NoError(t, err)
		assert.Len(t, info, 2)
		assert.Equal(t, "i-789", aws.ToString(info["i-789"].InstanceId))
		assert.Equal(t, "i-123", aws.ToString(info["i-123"].InstanceId))
	})

	t.Run("no IDs does not describe every instance", func(t *testing.T) {
		mockSvc := &mocks.MockEC2Client{}
		info, err := GetEC2InstanceInfo(ctx, mockSvc, nil)

		assert.NoError(t, err)
		assert.Empty(t, info)
		assert.Equal(t, 0, mockSvc.DescribeInstancesCalls())
	})

	t.Run("API errors are returned", func(t *testing.T) {
		mockSvc := &mocks.MockEC2Client{Err: mocks.NewAPIError("UnauthorizedOperation", "denied")}
		_, err := GetEC2InstanceInfo(ctx, mockSvc, []string{"i-123"})

		assert.ErrorContains(t, err, "UnauthorizedOperation")
	})
}
