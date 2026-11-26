package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"

	"github.com/bpal410/ssm-helpers/aws/resolver"
	"github.com/bpal410/ssm-helpers/aws/session"
	"github.com/bpal410/ssm-helpers/cmd/cmdutil"
	"github.com/bpal410/ssm-helpers/internal/tmux"
	ssmx "github.com/bpal410/ssm-helpers/ssm"
	"github.com/bpal410/ssm-helpers/ssm/instance"
)

func newCommandSSMSession() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "open a terminal session to an instance using SSM",
		Long:  "foo bar baz",
		Run: func(cmd *cobra.Command, args []string) {
			startSessionCommand(cmd, args)
		},
	}

	addBaseFlags(cmd)
	addSessionFlags(cmd)

	return cmd
}

func startSessionCommand(cmd *cobra.Command, args []string) {
	var err error
	var instanceList, addressList, profileList, regionList, tagList, attributeList []string

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

	var filterList map[string]string
	if filterList, err = cmdutil.GetMapFromStringSlice(cmd, "filter"); err != nil {
		log.Fatal(err)
	}

	if err = validateSessionFlags(cmd, instanceList, filterList); err != nil {
		log.Fatal(err)
	}

	if profileList, err = getProfileList(cmd); err != nil {
		log.Fatal(err)
	}
	if regionList, err = getRegionList(cmd); err != nil {
		log.Fatal(err)
	}
	if tagList, err = cmdutil.GetFlagStringSlice(cmd, "tag"); err != nil {
		log.Fatal(err)
	}
	if attributeList, err = cmdutil.GetFlagStringSlice(cmd, "attribute"); err != nil {
		log.Fatal(err)
	}

	var dryRunFlag bool
	if dryRunFlag, err = cmdutil.GetFlagBool(cmd, "dry-run"); err != nil {
		log.Fatal(err)
	}

	var sortTag string
	if sortTag, err = cmdutil.GetFlagString(cmd, "sort-tag"); err != nil {
		log.Fatal(err)
	}

	var sessionName string
	if sessionName, err = cmdutil.GetFlagString(cmd, "session-name"); err != nil {
		log.Fatal(err)
	}

	var limitFlag int
	if limitFlag, err = cmdutil.GetFlagInt(cmd, "limit"); err != nil {
		log.Fatal(err)
	}

	// Get the number of cores available for parallelization
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Create threadsafe pool of instance info to use for selection
	instancePool := instance.InstanceInfoSafe{
		AllInstances: make(map[string]instance.InstanceInfo),
	}

	var totalInstances int32
	var wg sync.WaitGroup

	ctx := cmd.Context()

	// Set up our AWS session for each permutation of profile + region and iterate over them
	sessionPool := session.NewPool(ctx, profileList, regionList, log)
	for _, sess := range sessionPool.Sessions {
		wg.Add(1)
		go func(sess *session.Session, instancePool *instance.InstanceInfoSafe) {
			defer wg.Done()
			threadLocalInstanceList := slices.Clone(instanceList)

			ssmClient := ssm.NewFromConfig(sess.Config)
			ec2Client := ec2.NewFromConfig(sess.Config)

			if len(addressList) > 0 {
				hr := resolver.NewHostnameResolver(addressList)
				ids, _ := hr.ResolveToInstanceId(ctx, ec2Client)
				threadLocalInstanceList = append(threadLocalInstanceList, ids...)
			}

			// Create our instance input object (filters, instances)
			diiInput := ssmx.CreateSSMDescribeInstanceInput(filterList, threadLocalInstanceList)

			sessionInstances, err := instance.GetSessionInstances(ctx, ssmClient, diiInput)
			if err != nil {
				log.Tracef("AWS Session Parameters: %s, %s", sess.Region(), sess.ProfileName)
				log.Fatal(err)
			}

			atomic.AddInt32(&totalInstances, int32(len(sessionInstances)))
			ssmx.CheckInstanceReadiness(ctx, sess, ssmClient, ec2Client, sessionInstances, limitFlag, instancePool)
		}(sess, &instancePool)
	}

	wg.Wait()

	log.Infof("Retrieved %d usable instances.", len(instancePool.AllInstances))

	// No functional results, exit now
	if len(instancePool.AllInstances) == 0 || dryRunFlag {
		return
	}

	// Single instance specified with --instance, starting session in current terminal (non-multiplexed).
	// A single instance found by searching goes to the selection prompt instead, so the user can confirm it.
	if len(instanceList) == 1 && len(instancePool.AllInstances) == 1 {
		for _, v := range instancePool.AllInstances {
			if err := startSSMSession(v.Profile, v.Region, v.InstanceID); err != nil {
				log.Errorf("Failed to start ssm-session for instance %s\n%s", v.InstanceID, err)
			}
		}
		return
	}

	var tmuxSession *tmux.Session

	// Multiple instances specified or found, check to see if we're in a tmux session to avoid nesting
	if len(instanceList) > 1 && len(instancePool.AllInstances) > 1 {
		var instances []instance.InstanceInfo
		for _, v := range instancePool.AllInstances {
			instances = append(instances, v)
		}

		if tmuxSession, err = configTmuxSession(sessionName, instances); err != nil {
			log.Fatal(err)
		}
	} else {
		// If -i was not specified, go to a selection prompt before starting sessions
		selectedInstances, err := startSelectionPrompt(&instancePool, totalInstances, sortTag, tagList, attributeList)
		if err != nil {
			if err == terminal.InterruptErr {
				log.Info("Instance selection interrupted.")
				os.Exit(0)
			}
			log.Errorf("Error during instance selection\n%s", err)
			os.Exit(1)
		}

		// If only one instance was selected, don't bother with a tmux session
		if len(selectedInstances) == 1 {
			v := selectedInstances[0]
			if err := startSSMSession(v.Profile, v.Region, v.InstanceID); err != nil {
				log.Fatalf("Failed to start session for instance %s\n%s", v.InstanceID, err)
			}
			return
		}

		if tmuxSession, err = configTmuxSession(sessionName, selectedInstances); err != nil {
			log.Fatal(err)
		}
	}

	// Make sure we aren't going to nest tmux sessions
	currentTmuxSocket := os.Getenv("TMUX")
	if len(currentTmuxSocket) == 0 {
		if err := tmuxSession.Attach(); err != nil {
			log.Errorf("Could not attach to tmux session '%s'\n%s", tmuxSession.Name, err)
		}
	} else {
		log.Info("To force nested tmux sessions, unset $TMUX.")
		log.Infof("Attach to the session with `tmux attach -t '=%s'`", tmuxSession.Name)
	}
}

func configTmuxSession(sessionName string, selectedInstances []instance.InstanceInfo) (*tmux.Session, error) {
	// Create the session (replacing any existing one with the same name) and
	// the window in which our ssm session panes will live. The window starts
	// with one empty pane, which we split from and remove at the end.
	tmuxSession, emptyPane, err := tmux.NewSession(sessionName, "ssm")
	if err != nil {
		return nil, fmt.Errorf("Failed to create tmux session\n%s", err)
	}

	// Configure our session-specific settings
	options := [][2]string{
		{"pane-border-status", "top"},
		{"mouse", "on"},
	}

	for _, o := range options {
		if err = tmuxSession.SetOption(o[0], o[1]); err != nil {
			return nil, fmt.Errorf("Failed to set tmux option %s\n%s", o[0], err)
		}
	}

	// Add a pane per instance, each below the previous one so they stay in order
	lastPane := emptyPane
	for _, v := range selectedInstances {
		if lastPane, err = addInstanceToTmuxWindow(lastPane, v.Profile, v.Region, v.InstanceID); err != nil {
			return nil, fmt.Errorf("Failed to add instance %s to tmux session\n%s", v.InstanceID, err)
		}

		// Re-tile our layout after each pane to avoid the "pane too small" error
		if err = tmuxSession.Tile(); err != nil {
			return nil, fmt.Errorf("Failed to re-tile panes\n%s", err)
		}
	}

	if err = emptyPane.Kill(); err != nil {
		return nil, fmt.Errorf("Failed to remove empty pane\n%s", err)
	}

	// Re-tile our layout one last time since we removed the empty pane
	if err = tmuxSession.Tile(); err != nil {
		return nil, fmt.Errorf("Failed to re-tile panes\n%s", err)
	}

	return tmuxSession, nil
}

func startSSMSession(profile string, region string, instanceID string) error {
	rawCmd := exec.Command("aws", "ssm", "start-session", "--profile", profile, "--region", region, "--target", instanceID)
	rawCmd.Stdin = os.Stdin
	rawCmd.Stdout = os.Stdout
	rawCmd.Stderr = os.Stderr

	err := rawCmd.Start()
	if err != nil {
		return err
	}

	// Set up to capture Ctrl+C
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	doneChan := make(chan struct{}, 2)

	// Run Wait() in its own chan so we don't block
	go func() {
		err = rawCmd.Wait()
		doneChan <- struct{}{}
	}()
	// Here we block until command is done
	for {
		select {
		case s := <-sigChan:
			// user typed Ctrl-C, most likley meant for ssm-session pass through
			rawCmd.Process.Signal(s)
		case <-doneChan:
			// command is done
			return err
		}
	}
}

// addInstanceToTmuxWindow splits a new pane below the given one and starts an
// SSM session to the instance in it.
func addInstanceToTmuxWindow(below tmux.Pane, profile string, region string, instanceID string) (tmux.Pane, error) {
	pane, err := below.Split()
	if err != nil {
		return tmux.Pane{}, err
	}

	if err = pane.SetTitle(instanceID); err != nil {
		return tmux.Pane{}, err
	}

	return pane, pane.SendLine(fmt.Sprintf("aws ssm start-session --profile %s --region %s --target %s", profile, region, instanceID))
}

func startSelectionPrompt(instances *instance.InstanceInfoSafe, totalInstances int32, sortTag string, tags, attributes ssmx.ListSlice) (selectedInstances []instance.InstanceInfo, err error) {
	instanceIDList := []string{}
	fields := append([]string(tags), []string(attributes)...)
	promptList := instances.FormatStringSlice(sortTag, fields...)
	fmt.Println("      ", promptList[0])

	prompt := &survey.MultiSelect{
		Message: fmt.Sprintf("Showing %d/%d instances. Make a Selection:", len(instances.AllInstances), totalInstances),
		Options: promptList[1 : len(promptList)-1],
	}

	if err := survey.AskOne(prompt, &instanceIDList, survey.WithPageSize(25)); err != nil {
		return nil, err
	}

	if len(instanceIDList) == 0 {
		return nil, fmt.Errorf("No instances selected")
	}

	// This is clunky, but currently necessary in order to finish creating the tmux sessions
	for _, v := range instanceIDList {
		id := strings.Split(v, " ")[0]
		if id != "" {
			selectedInstances = append(selectedInstances, instances.AllInstances[id])
		}
	}
	return selectedInstances, nil
}
