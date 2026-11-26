package cmd

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
)

func TestInputForSession(t *testing.T) {
	base := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  make([]string, 1, 10), // spare capacity, so a careless append would write into base
		Parameters:   map[string][]string{"commands": {"hostname"}},
	}
	base.InstanceIds[0] = "i-flag"

	accountA := inputForSession(base, []string{"i-aaa"})
	accountB := inputForSession(base, []string{"i-bbb"})

	// Each session gets its own instances, and the shared input is untouched
	assert.Equal(t, []string{"i-flag", "i-aaa"}, accountA.InstanceIds)
	assert.Equal(t, []string{"i-flag", "i-bbb"}, accountB.InstanceIds)
	assert.Equal(t, []string{"i-flag"}, base.InstanceIds)
	assert.NotSame(t, base, accountA)

	// Everything else is carried over
	assert.Equal(t, "AWS-RunShellScript", aws.ToString(accountA.DocumentName))
	assert.Equal(t, base.Parameters, accountA.Parameters)

	t.Run("no resolved IDs", func(t *testing.T) {
		input := inputForSession(&ssm.SendCommandInput{}, nil)
		assert.Empty(t, input.InstanceIds)
	})
}
