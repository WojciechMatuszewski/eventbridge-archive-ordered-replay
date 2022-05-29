import {
  aws_events,
  aws_events_targets,
  aws_iam,
  aws_logs,
  aws_sqs,
  aws_stepfunctions,
  aws_stepfunctions_tasks,
  CfnOutput,
  Duration,
  Stack,
  StackProps
} from "aws-cdk-lib";
import * as aws_lambda_go from "@aws-cdk/aws-lambda-go-alpha";
import { Construct } from "constructs";
import { join } from "path";

export class EBArchiveReplayStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const eventBus = new aws_events.EventBus(this, "EventBus");
    new CfnOutput(this, "eventBusName", {
      value: eventBus.eventBusName
    });
    new CfnOutput(this, "eventBusArn", {
      value: eventBus.eventBusArn
    });

    const eventBusArchive = new aws_events.Archive(this, "Archive2", {
      eventPattern: {
        source: ["eb-test-app"]
      },
      sourceEventBus: eventBus,
      retention: Duration.days(1)
    });
    new CfnOutput(this, "eventBusArchiveName", {
      value: eventBusArchive.archiveName
    });
    new CfnOutput(this, "eventBusArchiveArn", {
      value: eventBusArchive.archiveArn
    });

    const calculateWaitTimeFunction = new aws_lambda_go.GoFunction(
      this,
      "CalculateWaitTimeFunction",
      {
        entry: join(__dirname, "../src/calculate-wait-time")
      }
    );
    const calculateWaitTimeTask = new aws_stepfunctions_tasks.LambdaInvoke(
      this,
      "CalculateWaitTime",
      {
        lambdaFunction: calculateWaitTimeFunction,
        payloadResponseOnly: true,
        resultPath: "$.waitTimeInSeconds"
      }
    );
    const waitForWaitTimeTask = new aws_stepfunctions.Wait(
      this,
      "WaitForWaitTime",
      {
        time: aws_stepfunctions.WaitTime.secondsPath("$.waitTimeInSeconds")
      }
    );

    const putEventsTask = new aws_stepfunctions_tasks.CallAwsService(
      this,
      "PutEvents",
      {
        action: "putEvents",
        parameters: {
          Entries: [
            {
              "Detail.$": "$.originalEvent.detail",
              "DetailType.$": "$.originalEvent.detail-type",
              EventBusName: eventBus.eventBusArn,
              "Source.$": "$.originalEvent.source",
              "Time.$": "$.originalEvent.time"
            }
          ]
        },
        service: "eventbridge",
        iamResources: [eventBus.eventBusArn],
        iamAction: "events:PutEvents"
      }
    );

    const putEventsTaskFailedChoice = new aws_stepfunctions.Choice(
      this,
      "hasPutEventsFailed"
    )
      .when(
        aws_stepfunctions.Condition.numberGreaterThan("$.FailedEntryCount", 0),
        new aws_stepfunctions.Fail(this, "PutEventsFailed")
      )
      .otherwise(new aws_stepfunctions.Succeed(this, "PutEventsSucceeded"));

    const replayStateMachine = new aws_stepfunctions.StateMachine(
      this,
      "ReplayStateMachine",
      {
        definition: calculateWaitTimeTask
          .next(waitForWaitTimeTask)
          .next(putEventsTask)
          .next(putEventsTaskFailedChoice)
      }
    );
    new CfnOutput(this, "replayStateMachineArn", {
      value: replayStateMachine.stateMachineArn
    });

    const replayRuleRole = new aws_iam.Role(this, "ReplayRuleRole", {
      assumedBy: new aws_iam.ServicePrincipal("events.amazonaws.com"),
      inlinePolicies: {
        allowSFNInvoke: new aws_iam.PolicyDocument({
          statements: [
            new aws_iam.PolicyStatement({
              effect: aws_iam.Effect.ALLOW,
              actions: ["states:StartExecution"],
              resources: [replayStateMachine.stateMachineArn]
            })
          ]
        })
      }
    });
    new CfnOutput(this, "replayRuleRoleArn", {
      value: replayRuleRole.roleArn
    });

    const replayRule = new aws_events.Rule(this, "ReplayRule", {
      enabled: true,
      eventBus: eventBus,
      eventPattern: {
        source: ["eb-test-app"],
        // @ts-ignore
        "replay-name": [{ exists: true }]
      }
    });
    new CfnOutput(this, "replayRuleName", {
      value: replayRule.ruleName
    });

    const eventsTarget = new aws_logs.LogGroup(this, "LogGroup", {
      logGroupName: "/aws/events/EBTarget",
      retention: aws_logs.RetentionDays.ONE_DAY
    });

    const dlq = new aws_sqs.Queue(this, "TargetDLQ");
    new aws_events.Rule(this, "Rule", {
      description: "Forwards the events to the log group",
      enabled: true,
      eventBus,
      eventPattern: {
        source: ["eb-test-app"]
      },
      targets: [
        new aws_events_targets.CloudWatchLogGroup(eventsTarget, {
          retryAttempts: 0,
          deadLetterQueue: dlq,
          /**
           * See https://github.com/WojciechMatuszewski/eb-events-to-cw-logs-formatting
           */
          event: aws_events.RuleTargetInput.fromObject({
            timestamp: aws_events.EventField.time,
            message: `{\"id\": ${aws_events.EventField.fromPath(
              "$.detail.id"
            )}}`
          })
        })
      ]
    });
  }
}
