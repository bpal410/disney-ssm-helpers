package ec2

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GetEC2InstanceInfo describes the given EC2 instances and returns them keyed by instance ID.
// Instances that EC2 did not return are absent from the map.
func GetEC2InstanceInfo(ctx context.Context, client ec2.DescribeInstancesAPIClient, instanceIDs []string) (map[string]ec2types.Instance, error) {
	instances := make(map[string]ec2types.Instance, len(instanceIDs))

	// DescribeInstances with no IDs describes every instance in the account
	if len(instanceIDs) == 0 {
		return instances, nil
	}

	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("Could not describe EC2 instances\n%w", err)
		}

		for _, reservation := range page.Reservations {
			for _, i := range reservation.Instances {
				instances[aws.ToString(i.InstanceId)] = i
			}
		}
	}

	return instances, nil
}
