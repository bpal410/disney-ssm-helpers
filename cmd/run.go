package cmd

import (
	"context"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/bpal410/ssm-helpers/aws/resolver"
	"github.com/bpal410/ssm-helpers/aws/session"
	"github.com/bpal410/ssm-helpers/cmd/cmdutil"
	ssmx "github.com/bpal410/ssm-helpers/ssm"
	"github.com/bpal410/ssm-helpers/ssm/invocation"
)

func newCommandSSMRun() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "execute commands using the AWS-RunShellScript document",
		Long:  "foo bar baz",
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args)
		},
	}

	addBaseFlags(cmd)
	addRunFlags(cmd)

	return cmd
}

func runCommand(cmd *cobra.Command, args []string) {
	var err error
	var instanceList, addressList, commandList, profileList, regionList []string
	var maxConcurrency, maxErrors string
	var targets []ssmtypes.Target

	// Get all of our CLI flag values
	if err = cmdutil.ValidateArgs(cmd, args); err != nil {
		log.Fatal(err)
	}

	if instanceList, err = cmdutil.GetFlagStringSlice(cmd, "instance"); err != nil {
		log.Fatal(err)
	}
	if addressList, err = cmdutil.GetFlagStringSlice(cmd, "address"); err != nil {
		log.Fatal(err)
	}
	if commandList, err = getCommandList(cmd); err != nil {
		log.Fatal(err)
	}
	if targets, err = getTargetList(cmd); err != nil {
		log.Fatal(err)
	}

	if err := validateRunFlags(cmd, instanceList, addressList, commandList, targets); err != nil {
		log.Fatal(err)
	}

	if profileList, err = getProfileList(cmd); err != nil {
		log.Fatal(err)
	}
	if regionList, err = getRegionList(cmd); err != nil {
		log.Fatal(err)
	}

	if maxConcurrency, err = getMaxConcurrency(cmd); err != nil {
		log.Fatal(err)
	}
	if maxErrors, err = getMaxErrors(cmd); err != nil {
		log.Fatal(err)
	}

	// Get the number of cores available for parallelization
	runtime.GOMAXPROCS(runtime.NumCPU())

	log.Info("Command(s) to be executed:\n", strings.Join(commandList, "\n"))

	sciInput := &ssm.SendCommandInput{
		InstanceIds:  instanceList,
		Targets:      targets,
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			/*
				ssm.SendCommandInput objects require parameters for the DocumentName chosen

				For AWS-RunShellScript, the only required parameter is "commands",
				which is the shell command to be executed on the target. To emulate
				the original script, we also set "executionTimeout" to 10 minutes.
			*/
			"commands":         commandList,
			"executionTimeout": {"600"},
		},
		MaxConcurrency: aws.String(maxConcurrency),
		MaxErrors:      aws.String(maxErrors),
	}

	ctx := cmd.Context()
	wg, output := sync.WaitGroup{}, invocation.ResultSafe{}
	started := 0

	// Set up our AWS session for each permutation of profile + region and iterate over them
	sessionPool := newSessionPool(ctx, profileList, regionList)
	for _, sess := range sessionPool.Sessions {
		var resolvedIDs []string
		if len(addressList) > 0 {
			ec2Client := ec2.NewFromConfig(sess.Config)
			hr := resolver.NewHostnameResolver(addressList)
			if resolvedIDs, err = hr.ResolveToInstanceId(ctx, ec2Client); err != nil {
				log.Errorf("Could not resolve addresses to instances for %s in %s\n%v", sess.ProfileName, sess.Region(), err)
			}
		}

		sessionInput := inputForSession(sciInput, resolvedIDs)
		if len(sessionInput.InstanceIds) == 0 && len(sessionInput.Targets) == 0 {
			log.Debugf("No instances to target for %s in %s, skipping", sess.ProfileName, sess.Region())
			continue
		}

		ssmClient := ssm.NewFromConfig(sess.Config)

		log.Debugf("Starting invocation targeting account %s in %s", sess.ProfileName, sess.Region())
		started++
		wg.Add(1)
		go func() {
			defer wg.Done()
			ssmx.RunInvocations(ctx, sess, ssmClient, sessionInput, &output)
		}()
	}

	wg.Wait() // Wait for each account/region combo to finish

	// Only possible when --address matched no instances in any profile or region
	if started == 0 {
		log.Fatal("No instances matched the given addresses in any profile or region")
	}

	resultFormat := "%-24s %-15s %-15s %s"
	var successCounter, failedCounter int

	// Output our results
	log.Infof(resultFormat, "Instance ID", "Region", "Profile", "Status")
	for _, v := range output.InvocationResults {

		if v.Status == invocation.ClientError {
			log.Errorf(resultFormat, "---", v.Region, v.ProfileName, v.Status)
			failedCounter++
			continue
		}

		if v.Status == invocation.CommandSuccess {
			log.Infof(resultFormat, aws.ToString(v.InvocationResult.InstanceId), v.Region, v.ProfileName, v.Status)
			successCounter++
		} else {
			log.Errorf(resultFormat, aws.ToString(v.InvocationResult.InstanceId), v.Region, v.ProfileName, v.Status)
			failedCounter++
		}

		// stdout is always written back at info level
		if stdout := aws.ToString(v.InvocationResult.StandardOutputContent); stdout != "" {
			log.Info(stdout)
		}

		// stderr is written back at warn if the invocation was successful, and error if not
		if stderr := aws.ToString(v.InvocationResult.StandardErrorContent); stderr != "" {
			if v.Status == invocation.CommandSuccess {
				log.Warn(stderr)
			} else {
				log.Error(stderr)
			}
		}
	}

	log.Infof("Execution results: %d SUCCESS, %d FAILED", successCounter, failedCounter)
	if failedCounter > 0 { // Exit code 1 to indicate that there was some sort of error returned from invocation
		os.Exit(1)
	}

	return
}

// inputForSession returns a copy of base for one profile/region, with that
// session's resolved instance IDs added. base is shared by every session and
// is never modified, so instance IDs found in one account are not sent to another.
func inputForSession(base *ssm.SendCommandInput, resolvedIDs []string) *ssm.SendCommandInput {
	input := *base
	input.InstanceIds = append(slices.Clone(base.InstanceIds), resolvedIDs...)
	return &input
}

// newSessionPool creates the AWS sessions for every profile/region pair. Pairs
// that fail are logged and skipped; if none succeed, the command exits.
func newSessionPool(ctx context.Context, profiles []string, regions []string) *session.Pool {
	pool, err := session.NewPool(ctx, profiles, regions, log)
	if err != nil {
		if len(pool.Sessions) == 0 {
			log.Fatalf("Error when trying to create session:\n%v", err)
		}
		log.Errorf("Some profile/region combinations will be skipped:\n%v", err)
	}
	return pool
}
