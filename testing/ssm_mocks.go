package mocks

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
)

// MockSSMClient implements the parts of the SSM API used by this tool.
//
// GetConnectionStatus answers from ConnectionStatus (instance ID -> status) and
// ConnectionErrors (instance ID -> error); instances in neither are connected.
// SendCommand records each input in SentCommands.
type MockSSMClient struct {
	ConnectionStatus map[string]ssmtypes.ConnectionStatus
	ConnectionErrors map[string]error

	mu           sync.Mutex
	SentCommands []*ssm.SendCommandInput
}

// NewAPIError returns an error shaped like one returned by an AWS API
func NewAPIError(code, message string) error {
	return &smithy.GenericAPIError{Code: code, Message: message}
}

func (m *MockSSMClient) GetConnectionStatus(_ context.Context, input *ssm.GetConnectionStatusInput, _ ...func(*ssm.Options)) (*ssm.GetConnectionStatusOutput, error) {
	target := aws.ToString(input.Target)

	if err, ok := m.ConnectionErrors[target]; ok {
		return nil, err
	}

	status := ssmtypes.ConnectionStatusConnected
	if s, ok := m.ConnectionStatus[target]; ok {
		status = s
	}

	return &ssm.GetConnectionStatusOutput{Target: input.Target, Status: status}, nil
}

func (m *MockSSMClient) GetCommandInvocation(_ context.Context, input *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	var status string

	switch aws.ToString(input.CommandId) {
	case "success-id":
		status = "Success"
	case "failed-id", "mixed-id":
		status = "Failed"
	case "pending-id":
		status = "Pending"
	case "bad-id":
		return nil, NewAPIError("InvalidCommandId", "InvalidCommandId")
	}

	return &ssm.GetCommandInvocationOutput{
		InstanceId:            input.InstanceId,
		CommandId:             input.CommandId,
		StatusDetails:         aws.String(status),
		StandardOutputContent: aws.String(""),
		StandardErrorContent:  aws.String(""),
	}, nil
}

func (m *MockSSMClient) SendCommand(_ context.Context, input *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	if len(input.InstanceIds) > 0 && len(input.Targets) > 0 {
		return nil, fmt.Errorf("Cannot specify instance IDs and SSM targets in same SendCommandInput")
	}

	m.mu.Lock()
	m.SentCommands = append(m.SentCommands, input)
	m.mu.Unlock()

	// Mock our response from the SSM API
	return &ssm.SendCommandOutput{
		Command: &ssmtypes.Command{
			CommandId:    aws.String("success-id"),
			DocumentName: input.DocumentName,
			InstanceIds:  input.InstanceIds,
			Parameters:   input.Parameters,
			Targets:      input.Targets,
		},
	}, nil
}

// ListCommands reports every command as finished successfully
func (m *MockSSMClient) ListCommands(_ context.Context, input *ssm.ListCommandsInput, _ ...func(*ssm.Options)) (*ssm.ListCommandsOutput, error) {
	return &ssm.ListCommandsOutput{
		Commands: []ssmtypes.Command{
			{CommandId: input.CommandId, Status: ssmtypes.CommandStatusSuccess},
		},
	}, nil
}

// ListCommandInvocations returns one invocation per instance in the most recent
// SendCommand, one instance per page so that paging is exercised.
func (m *MockSSMClient) ListCommandInvocations(_ context.Context, input *ssm.ListCommandInvocationsInput, _ ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
	m.mu.Lock()
	var instanceIDs []string
	if len(m.SentCommands) > 0 {
		instanceIDs = m.SentCommands[len(m.SentCommands)-1].InstanceIds
	}
	m.mu.Unlock()

	page := 0
	if input.NextToken != nil {
		page, _ = strconv.Atoi(*input.NextToken)
	}
	if page >= len(instanceIDs) {
		return &ssm.ListCommandInvocationsOutput{}, nil
	}

	output := &ssm.ListCommandInvocationsOutput{
		CommandInvocations: []ssmtypes.CommandInvocation{
			{CommandId: input.CommandId, InstanceId: aws.String(instanceIDs[page])},
		},
	}
	if page+1 < len(instanceIDs) {
		output.NextToken = aws.String(strconv.Itoa(page + 1))
	}

	return output, nil
}

func (m *MockSSMClient) DescribeInstanceInformation(_ context.Context, input *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	// Mock our response from the SSM API, split over two pages
	if input.NextToken == nil {
		output := &ssm.DescribeInstanceInformationOutput{
			InstanceInformationList: []ssmtypes.InstanceInformation{
				{
					PlatformType:    ssmtypes.PlatformTypeLinux,
					PingStatus:      ssmtypes.PingStatusConnectionLost,
					InstanceId:      aws.String("i-23456"),
					IsLatestVersion: aws.Bool(true),
				},
				{
					PlatformType:    ssmtypes.PlatformTypeLinux,
					PingStatus:      ssmtypes.PingStatusOnline,
					InstanceId:      aws.String("i-45678"),
					IsLatestVersion: aws.Bool(true),
				},
				{
					PlatformType:    ssmtypes.PlatformTypeWindows,
					PingStatus:      ssmtypes.PingStatusConnectionLost,
					InstanceId:      aws.String("i-78901"),
					IsLatestVersion: aws.Bool(true),
				},
				{
					PlatformType:    ssmtypes.PlatformTypeLinux,
					PingStatus:      ssmtypes.PingStatusOnline,
					InstanceId:      aws.String("i-98765"),
					IsLatestVersion: aws.Bool(false),
				},
			},
			NextToken: aws.String("eyJNYXJrZXIiOiBudWxsLCAiYm90b190cnVuY2F0ZV9hbW91bnQiOiAxfQ=="),
		}
		return filterDescribeInstanceInformationOutput(input, output), nil
	}

	output := &ssm.DescribeInstanceInformationOutput{
		InstanceInformationList: []ssmtypes.InstanceInformation{
			{
				PlatformType:    ssmtypes.PlatformTypeLinux,
				PingStatus:      ssmtypes.PingStatusOnline,
				InstanceId:      aws.String("i-12345"),
				IsLatestVersion: aws.Bool(true),
			},
			{
				PlatformType:    ssmtypes.PlatformTypeLinux,
				PingStatus:      ssmtypes.PingStatusOnline,
				InstanceId:      aws.String("i-34567"),
				IsLatestVersion: aws.Bool(true),
			},
			{
				PlatformType:    ssmtypes.PlatformTypeWindows,
				PingStatus:      ssmtypes.PingStatusOnline,
				InstanceId:      aws.String("i-67890"),
				IsLatestVersion: aws.Bool(true),
			},
		},
		NextToken: nil,
	}

	return filterDescribeInstanceInformationOutput(input, output), nil
}

// filterDescribeInstanceInformationOutput keeps only the instances that match every filter in the input
func filterDescribeInstanceInformationOutput(input *ssm.DescribeInstanceInformationInput, output *ssm.DescribeInstanceInformationOutput) *ssm.DescribeInstanceInformationOutput {
	filtered := []ssmtypes.InstanceInformation{}

	for _, instance := range output.InstanceInformationList {
		match := true
		for _, filter := range input.Filters {
			if !slices.Contains(filter.Values, instanceFieldValue(instance, aws.ToString(filter.Key))) {
				match = false
				break
			}
		}

		if match {
			filtered = append(filtered, instance)
		}
	}

	output.InstanceInformationList = filtered
	return output
}

// instanceFieldValue returns the value of the instance field that a filter key refers to
func instanceFieldValue(instance ssmtypes.InstanceInformation, key string) string {
	switch key {
	case "PingStatus":
		return string(instance.PingStatus)
	case "PlatformType":
		return string(instance.PlatformType)
	case "IsLatestVersion":
		return strconv.FormatBool(aws.ToBool(instance.IsLatestVersion))
	case "InstanceIds":
		return aws.ToString(instance.InstanceId)
	default:
		return ""
	}
}
