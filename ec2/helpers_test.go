package ec2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	mocks "github.com/bpal410/ssm-helpers/testing"
)

func TestGetEC2InstanceInfo(t *testing.T) {
	t.Run("get instance info from IDs", func(t *testing.T) {
		mockSvc := &mocks.MockEC2Client{}
		info, err := GetEC2InstanceInfo(context.Background(), mockSvc, []string{"i-123", "i-456", "i-789"})

		assert.NoError(t, err)
		assert.Len(t, info, 3)
	})
}
