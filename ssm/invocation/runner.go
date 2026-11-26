package invocation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// CommandInvocationAPI is the part of the SSM API used to fetch an invocation's result
type CommandInvocationAPI interface {
	GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

// GetResult fetches the result of a command invocation on a single instance
func GetResult(ctx context.Context, client CommandInvocationAPI, commandID string, instanceID string) (*ssm.GetCommandInvocationOutput, error) {
	status, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
		CommandId:  aws.String(commandID),
		InstanceId: aws.String(instanceID),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"Error when calling GetCommandInvocation API with args:\nCommandId: %v\nInstanceId: %v\n%w",
			commandID, instanceID, err)
	}

	return status, nil
}
