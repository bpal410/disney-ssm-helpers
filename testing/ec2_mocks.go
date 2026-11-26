package mocks

import (
	"context"
	"slices"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// MockEC2Client implements the parts of the EC2 API used by this tool.
// If Err is set, DescribeInstances returns it.
type MockEC2Client struct {
	Err error

	describeInstancesCalls atomic.Int32
}

// DescribeInstancesCalls returns how many times DescribeInstances was called
func (m *MockEC2Client) DescribeInstancesCalls() int {
	return int(m.describeInstancesCalls.Load())
}

var mockEC2Instances = []ec2types.Instance{
	{
		InstanceId: aws.String("i-123"),
		VpcId:      aws.String("vpc-123"),
		Tags: []ec2types.Tag{
			{Key: aws.String("id_foo"), Value: aws.String("321")},
			{Key: aws.String("env"), Value: aws.String("dev")},
		},
	},
	{
		InstanceId: aws.String("i-456"),
		VpcId:      aws.String("vpc-456"),
		Tags: []ec2types.Tag{
			{Key: aws.String("id_bar"), Value: aws.String("654")},
			{Key: aws.String("env"), Value: aws.String("prod")},
		},
	},
	{
		InstanceId: aws.String("i-789"),
		VpcId:      aws.String("vpc-789"),
		Tags: []ec2types.Tag{
			{Key: aws.String("id_baz"), Value: aws.String("987")},
			{Key: aws.String("env"), Value: aws.String("qa")},
		},
	},
}

// DescribeInstances returns the mock instances whose IDs were requested, or all of them when none were
func (m *MockEC2Client) DescribeInstances(_ context.Context, input *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	m.describeInstancesCalls.Add(1)

	if m.Err != nil {
		return nil, m.Err
	}

	var instances []ec2types.Instance
	for _, i := range mockEC2Instances {
		if len(input.InstanceIds) == 0 || slices.Contains(input.InstanceIds, aws.ToString(i.InstanceId)) {
			instances = append(instances, i)
		}
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{Instances: instances}},
	}, nil
}
