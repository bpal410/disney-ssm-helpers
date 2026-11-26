package ssm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/bpal410/ssm-helpers/aws/session"
	ec2helpers "github.com/bpal410/ssm-helpers/ec2"
	"github.com/bpal410/ssm-helpers/ssm/instance"
	"github.com/bpal410/ssm-helpers/ssm/invocation"
	startsession "github.com/bpal410/ssm-helpers/ssm/session"
)

// CommandAPI is the part of the SSM API used to run commands and collect their results
type CommandAPI interface {
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	ListCommands(ctx context.Context, params *ssm.ListCommandsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandsOutput, error)
	ssm.ListCommandInvocationsAPIClient
	invocation.CommandInvocationAPI
}

// invocationPollInterval is how long to wait between checks of a running command's status
var invocationPollInterval = 2 * time.Second

// maxConsecutiveReadinessErrors stops readiness checks for a profile/region after this many
// errors in a row, which usually means a permissions problem rather than a bad instance
const maxConsecutiveReadinessErrors = 3

// CreateSSMDescribeInstanceInput returns an *ssm.DescribeInstanceInformationInput object for use when calling the DescribeInstanceInformation() method
func CreateSSMDescribeInstanceInput(filters map[string]string, instances CommaSlice) *ssm.DescribeInstanceInformationInput {
	var iisFilters []ssmtypes.InstanceInformationStringFilter

	// Build our filters based on --filter and --instances flags
	buildFilters(filters, &iisFilters)

	if len(instances) > 0 {
		AppendSSMFilter(&iisFilters, NewSSMInstanceFilter("InstanceIds", instances))
	}

	return &ssm.DescribeInstanceInformationInput{
		Filters: iisFilters,

		// Max number of results per page allowed by the API is 50
		MaxResults: aws.Int32(50),
	}
}

func addInstanceInfo(instanceID string, ec2Instance ec2types.Instance, instancePool *instance.InstanceInfoSafe, profile string, region string) {
	tags := make(map[string]string)
	for _, tag := range ec2Instance.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	instancePool.Lock()
	defer instancePool.Unlock()

	// If the instance is good, append its info to the master list
	instancePool.AllInstances[instanceID] = instance.InstanceInfo{
		InstanceID: instanceID,
		Profile:    profile,
		Region:     region,
		Tags:       tags,
		VpcId:      aws.ToString(ec2Instance.VpcId),
	}
}

func checkInvocationStatus(ctx context.Context, client CommandAPI, commandID string) (done bool, err error) {
	var invocation *ssm.ListCommandsOutput
	if invocation, err = client.ListCommands(ctx, &ssm.ListCommandsInput{
		CommandId: aws.String(commandID),
	}); err != nil {
		return true, fmt.Errorf("Encountered an error when trying to call the ListCommands API with CommandId: %v\n%w", commandID, err)
	}

	if len(invocation.Commands) != 1 {
		return true, fmt.Errorf("Incorrect number of invocations returned for given command ID; expected 1, got %d", len(invocation.Commands))
	}

	switch invocation.Commands[0].Status {
	case ssmtypes.CommandStatusPending, ssmtypes.CommandStatusInProgress, ssmtypes.CommandStatusCancelling:
		return false, nil
	default:
		return true, nil
	}
}

// RunInvocations invokes an SSM document with given parameters and adds the result for each targeted instance to results
func RunInvocations(ctx context.Context, sess *session.Session, client CommandAPI, input *ssm.SendCommandInput, results *invocation.ResultSafe) {
	// Send our command input to SSM
	scOutput, err := client.SendCommand(ctx, input)
	if err != nil {
		sess.Logger.Error("Error when calling the SendCommand API")
		addError(results, sess, err)
		return
	}

	commandID := aws.ToString(scOutput.Command.CommandId)
	sess.Logger.Infof("Started invocation %v for %v in %v", commandID, sess.ProfileName, sess.Region())

	// Watch status of invocation to see when it's done and we can get the output
	for {
		done, err := checkInvocationStatus(ctx, client, commandID)
		if err != nil {
			addError(results, sess, err)
			return
		}
		if done {
			break
		}

		select {
		case <-ctx.Done():
			addError(results, sess, ctx.Err())
			return
		case <-time.After(invocationPollInterval):
		}
	}

	// Fetch the results of our invocation for every instance it ran on
	paginator := ssm.NewListCommandInvocationsPaginator(client, &ssm.ListCommandInvocationsInput{
		CommandId: aws.String(commandID),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			sess.Logger.Error("Error when calling ListCommandInvocations API")
			addError(results, sess, err)
			return
		}

		for _, entry := range page.CommandInvocations {
			result, err := invocation.GetResult(ctx, client, commandID, aws.ToString(entry.InstanceId))
			if err != nil {
				addError(results, sess, err)
				continue
			}
			addInvocationResult(results, sess, result)
		}
	}
}

func addInvocationResult(results *invocation.ResultSafe, session *session.Session, info *ssm.GetCommandInvocationOutput) {
	results.Add(&invocation.Result{
		InvocationResult: info,
		ProfileName:      session.ProfileName,
		Region:           session.Region(),
		Status:           invocation.Status(aws.ToString(info.StatusDetails)),
	})
}

func addError(results *invocation.ResultSafe, session *session.Session, err error) {
	results.Add(&invocation.Result{
		ProfileName: session.ProfileName,
		Region:      session.Region(),
		Status:      invocation.ClientError,
		Error:       err,
	})
}

// CheckInstanceReadiness checks, in order, whether each instance is ready for a session, and adds
// up to limit ready instances (with their EC2 tags, where they have them) to readyInstancePool.
func CheckInstanceReadiness(ctx context.Context, sess *session.Session, ssmClient startsession.ConnectionStatusAPI, ec2Client ec2.DescribeInstancesAPIClient, instanceList []ssmtypes.InstanceInformation, limit int, readyInstancePool *instance.InstanceInfoSafe) {
	var readyInstances, ec2Instances []string
	consecutiveErrors := 0

	for _, i := range instanceList {
		if len(readyInstances) >= limit {
			break
		}

		instanceID := aws.ToString(i.InstanceId)

		// Check and see if our instance supports start-session
		ready, err := startsession.CheckSessionReadiness(ctx, ssmClient, instanceID)
		if err != nil {
			sess.Logger.Errorf("Error when trying to check session readiness for instance %v\n%v", instanceID, err)

			if consecutiveErrors++; consecutiveErrors >= maxConsecutiveReadinessErrors {
				sess.Logger.Errorf("Skipping remaining readiness checks for %s in %s after %d errors in a row", sess.ProfileName, sess.Region(), consecutiveErrors)
				break
			}
			continue
		}
		consecutiveErrors = 0

		if !ready {
			sess.Logger.Debugf("Instance %s is not connected to SSM, skipping", instanceID)
			continue
		}

		readyInstances = append(readyInstances, instanceID)

		// Managed (mi-) instances are not EC2 instances, so only EC2 instances have tags to fetch
		if !strings.HasPrefix(instanceID, "mi-") {
			ec2Instances = append(ec2Instances, instanceID)
		}
	}

	// Get the tags to display during instance selection
	ec2Info, err := ec2helpers.GetEC2InstanceInfo(ctx, ec2Client, ec2Instances)
	if err != nil {
		sess.Logger.Error(err)
	}

	for _, id := range readyInstances {
		// Instances without EC2 info (managed instances, or a failed lookup) are listed without tags
		addInstanceInfo(id, ec2Info[id], readyInstancePool, sess.ProfileName, sess.Region())
	}
}
