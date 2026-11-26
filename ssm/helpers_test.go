package ssm

import (
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bpal410/ssm-helpers/aws/session"
	"github.com/bpal410/ssm-helpers/ssm/instance"
	"github.com/bpal410/ssm-helpers/ssm/invocation"
	mocks "github.com/bpal410/ssm-helpers/testing"
)

func testSession() *session.Session {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return &session.Session{
		Logger:      logger,
		Config:      aws.Config{Region: "us-east-1"},
		ProfileName: "testprofile",
	}
}

func newPool() *instance.InstanceInfoSafe {
	return &instance.InstanceInfoSafe{AllInstances: map[string]instance.InstanceInfo{}}
}

func TestCreateSSMDescribeInstanceInput(t *testing.T) {
	assert := assert.New(t)

	filters := map[string]string{
		"foo": "1",
		"bar": "2",
		"baz": "3",
	}

	instances := &CommaSlice{"i-12345", "i-67890"}

	instanceInput := CreateSSMDescribeInstanceInput(filters, *instances)

	// Ensure that appending of filters is working correctly for multiple tags
	assert.Lenf(instanceInput.Filters, 4, "Filter slice has wrong number of entries, got %d, expected 4 items (instance IDs + all tags)", len(instanceInput.Filters))

	// Ensure that MaxResults is set to 50, which is the maximum number of results returned per page
	assert.Equal(int32(50), aws.ToInt32(instanceInput.MaxResults))
}

func TestAddInstanceInfo(t *testing.T) {
	assert := assert.New(t)

	id := "i-123"
	dummyInstance := ec2types.Instance{
		InstanceId: aws.String(id),
		Tags: []ec2types.Tag{
			{Key: aws.String("foo"), Value: aws.String("1")},
			{Key: aws.String("bar"), Value: aws.String("2")},
		},
		VpcId: aws.String("vpc-123"),
	}

	ip := newPool()
	addInstanceInfo(id, dummyInstance, ip, "testprofile", "us-east-1")

	assert.Equal(id, ip.AllInstances[id].InstanceID)
	assert.Equal("vpc-123", ip.AllInstances[id].VpcId)
	assert.Equal(map[string]string{"foo": "1", "bar": "2"}, ip.AllInstances[id].Tags)
}

func TestRunInvocations(t *testing.T) {
	invocationPollInterval = 0
	ctx := context.Background()
	sess := testSession()

	client := &mocks.MockSSMClient{}
	results := &invocation.ResultSafe{}

	RunInvocations(ctx, sess, client, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{"i-123", "i-456", "i-789"},
	}, results)

	// One result per instance, collected across every page of invocations
	require.Len(t, results.InvocationResults, 3)
	var ids []string
	for _, r := range results.InvocationResults {
		assert.Equal(t, invocation.CommandSuccess, r.Status)
		assert.Equal(t, "us-east-1", r.Region)
		assert.Equal(t, "testprofile", r.ProfileName)
		ids = append(ids, aws.ToString(r.InvocationResult.InstanceId))
	}
	assert.ElementsMatch(t, []string{"i-123", "i-456", "i-789"}, ids)
}
