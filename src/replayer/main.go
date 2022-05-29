package main

import (
	"app/env"
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("Starting replay")

	err := run(ctx)
	if err != nil {
		panic(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	client := eventbridge.NewFromConfig(cfg)

	eventBusArn, err := env.Get(env.EVENT_BUS_ARN)
	if err != nil {
		return err
	}
	eventBusArchiveArn, err := env.Get(env.EVENT_BUS_ARCHIVE_ARN)
	if err != nil {
		return err
	}
	replayStateMachineArn, err := env.Get(env.REPLAY_STATE_MACHINE_ARN)
	if err != nil {
		return err
	}
	replayRuleName, err := env.Get(env.REPLAY_RULE_NAME)
	if err != nil {
		return err
	}
	replayRuleRoleArn, err := env.Get(env.REPLAY_RULE_ROLE_ARN)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	startTime := now.Add(-time.Hour)

	fmt.Println("Replaying events starting from: ", startTime.Format(time.RFC3339))

	err = upsertRuleTarget(ctx, UpsertRuleTargetParams{
		Client:      client,
		EventBusArn: eventBusArn,
		RuleName:    replayRuleName,
		RuleRoleArn: replayRuleRoleArn,
		TargetArn:   replayStateMachineArn,
		StartTime:   startTime,
	})
	if err != nil {
		return err
	}

	replayName := strings.ReplaceAll(strings.ReplaceAll(now.Format(time.Stamp), " ", "-"), ":", ".")
	fmt.Println("Replay name: ", replayName)
	out, err := client.StartReplay(ctx, &eventbridge.StartReplayInput{
		ReplayName:     aws.String(replayName),
		EventStartTime: aws.Time(startTime),
		EventEndTime:   aws.Time(now),
		Destination: &eventbridgetypes.ReplayDestination{
			Arn: aws.String(eventBusArn),
		},
		EventSourceArn: aws.String(eventBusArchiveArn),
	})
	if err != nil {
		return err
	}

	replayArn := *out.ReplayArn
	if out.State == eventbridgetypes.ReplayStateCancelled {
		return fmt.Errorf("replay %s was cancelled", replayArn)
	}

	for {
		fmt.Println("Waiting for replay to finish")

		out, err := client.DescribeReplay(ctx, &eventbridge.DescribeReplayInput{
			ReplayName: aws.String(replayName),
		})
		if err != nil {
			return err
		}

		fmt.Printf("Replay %s is %s\n", *out.ReplayName, string(out.State))

		if out.State == eventbridgetypes.ReplayStateCancelled {
			return fmt.Errorf("replay %s was cancelled: %s", replayArn, *out.StateReason)
		}

		if out.State == eventbridgetypes.ReplayStateFailed {
			return fmt.Errorf("replay %s failed: %s", replayArn, *out.StateReason)
		}

		if out.State == eventbridgetypes.ReplayStateCompleted {
			return nil
		}

		time.Sleep(time.Second * 5)
	}
}

type UpsertRuleTargetParams struct {
	Client      *eventbridge.Client
	EventBusArn string
	RuleName    string
	RuleRoleArn string
	TargetArn   string
	StartTime   time.Time
}

// When you create or update a rule, incoming events might not immediately start matching to new or updated rules. Allow a short period of time for changes to take effect.
func upsertRuleTarget(ctx context.Context, params UpsertRuleTargetParams) error {
	fmt.Println("Upserting target for rule: ", params.RuleName)

	out, err := params.Client.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule:         aws.String(params.RuleName),
		EventBusName: aws.String(params.EventBusArn),
		Targets: []eventbridgetypes.Target{
			{
				Arn: aws.String(params.TargetArn),
				InputTransformer: &eventbridgetypes.InputTransformer{
					InputTemplate: aws.String(fmt.Sprintf("{\"originalEvent\": <originalEvent>, \"startTime\": \"%s\"}", params.StartTime.Format(time.RFC3339))),
					InputPathsMap: map[string]string{
						"originalEvent": "$",
					},
				},
				RetryPolicy: &eventbridgetypes.RetryPolicy{
					MaximumRetryAttempts: aws.Int32(0),
				},
				Id:      aws.String("rule"),
				RoleArn: aws.String(params.RuleRoleArn),
			},
		},
	})
	if err != nil {
		return err
	}

	if out.FailedEntryCount > 0 {
		return fmt.Errorf("failed to put targets: %v", out.FailedEntries)
	}

	fmt.Println("Target for rule: ", params.RuleName, "ready!")

	return nil
}
