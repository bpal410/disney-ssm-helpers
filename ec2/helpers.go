package ec2

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GetEC2InstanceInfo describes the given EC2 instances and returns them in the order requested
func GetEC2InstanceInfo(ctx context.Context, client ec2.DescribeInstancesAPIClient, instances []string) (output []ec2types.Instance, err error) {
	keyedInstances := make(map[string]ec2types.Instance)

	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		InstanceIds: instances,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("Could not describe EC2 instances\n%w", err)
		}

		for _, reservation := range page.Reservations {
			for _, i := range reservation.Instances {
				keyedInstances[aws.ToString(i.InstanceId)] = i
			}
		}
	}

	for _, id := range instances {
		output = append(output, keyedInstances[id])
	}

	return output, nil
}
