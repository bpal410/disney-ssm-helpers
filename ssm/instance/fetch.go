package instance

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func GetSessionInstances(ctx context.Context, client ssm.DescribeInstanceInformationAPIClient, diiInput *ssm.DescribeInstanceInformationInput) (output []ssmtypes.InstanceInformation, err error) {
	// Fetch all instances that match the provided filters
	paginator := ssm.NewDescribeInstanceInformationPaginator(client, diiInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("Could not retrieve SSM instance info\n%w", err)
		}
		output = append(output, page.InstanceInformationList...)
	}

	return output, nil
}
