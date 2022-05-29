# EventBridge archive replay and ordering

Understand the EventBridge archive replay and its implications on the order of replayed events.

This repository and the exploration of the EventBridge archive are inspired by the [eventbridge-cli](https://github.com/spezam/eventbridge-cli) and its "keep the order of events while replaying them" feature.

## The problem

Replaying events from the EventBridge archive does not respect the original order of the events ([Source](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-replay-archived-event.html)). It would be neat to be able to replay the events in the same order they were published.

How to do that?

## The solution

To ensure ordered delivery of the events, we have to have a way to "sort" the events. The solution uses a StepFunction and the `Wait` task to ensure the replied events are sent to the event bus in the **order based on the `time` event parameter, not the delivery order**.

First, we replay the events to a special rule that triggers a StepFunction. The StepFunction calculates the time a given event has to wait, then dispatches that event.

While very precise, this method is quite costly. **If you decide to go this route, the EventBridge will invoke a StepFunction for each event**. Depending on the number of events in the archive (and the specified start time of the replay), the number of StepFunction invocations might be pretty high.

---

I've looked at SQS and the _visibility timeout_ as the alternative, but after a bit of digging, I concluded the SQS service is unfit to work in this scenario.

The regular SQS queue does not guarantee message ordering upon retrieval of the events – even if we specify the correct visibility timeout (akin to what we are doing in the StepFunctions).

With the SQS Fifo, it **is impossible to fetch** another message from the queue with the same _Message Group ID_\*\*. Imagine your first message waiting there, blocking the second one (which should be delivered first).

## Learnings

- Guess who spent 2 hours sending the events to a bus that does not exist? Yup, me.

  - I'm not sure why the `PutEvents` API does not validate the `EventBusName` parameter. **It will accept the event as long as its structure is valid**.

  - Initially, I thought this might be the case only for the SDK – maybe something funky is going on within the auto-generated Smithy code? But nope, the CLI behaves the same way.

    The [documentation](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_PutEventsRequestEntry.html#eventbridge-Type-PutEventsRequestEntry-EventBusName) mentions that the default bus is used if the `EventBusName` parameter is not defined. Again, this is not the case for the `EventBusName` that does not exist.

  - I've [twitted about this "issue" (?)](https://twitter.com/wm_matuszewski/status/1528342818120507392).

- I forgot about the EventBridge -> CloudWatch Logs Group formatting problem when using the _Input Transformer_. Luckily I [published an example of how to deal [with it](https://github.com/WojciechMatuszewski/eb-events-to-cw-logs-formatting), so I had some reference material :).

- I could not find a way to _purge_ the EventBridge archive. Quite disappointing, to be honest.

- When you replay the events, the events are not lost. The **archive preserves the events unless they reach their retention period**. You can **replay the same events multiple times**.

  - Great for production use-cases, but not so great for testing ones :p

  - The **replied events have a unique "replay-name" property appended to them**. A convenient thing if you want to target only the events from the replay.

  - The AWS CDK `aws_events` package filtering expressions do not support the `replay-name` attribute. I had to use `@ts-ignore` to apply the right filtering pattern.

- Just like the delivery of the EventBridge events, the **EventBridge Archive takes some time to ingest the events**. You will most likely need to wait a bit for the events to show up in the archive before you can replay them!

  - You can learn more about this behavior [in this article](https://medium.com/lego-engineering/amazon-eventbridge-archive-replay-an-experience-report-6aabc744df5a).

- You **cannot control the rate the archive sends the events to the given target**.

  - One **solution** would be to use **an intermediatory SQS queue**.

    - [This article](https://medium.com/lego-engineering/amazon-eventbridge-archive-replay-events-in-tandem-with-a-circuit-breaker-c049a4c6857f) describes how to do that.

- I could not find a way to dynamically inject the time the replay started into the replayed event using CDK.

  - There is the `aws.events.event.ingestion-time` predefined variable, but that one would be different for each event, which is NOT what I want.

  - The **solution** is to **create the rule via the CDK and update the rule target dynamically**.

- One might use the `RoleArn` property on the _rule_ or the _target_ level.

  - The one at the _target_ level allows for granularity (a separate role for each target), while the one at the _rule_ level is the "default" one.

  - The API interface is misleading as it denotes the `RoleArn` for the `PutTargets` call as optional. That is not the case when the target is a StepFunction. Consult the [documentation](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_PutTargets.html) to learn more.

  - More reading [here](https://github.com/aws-cloudformation/cloudformation-coverage-roadmap/issues/352).

- When using the `InputsPathMap`, remember that **each property has to have a VALID JSON PATH value**. It cannot be a static value. Those should go directly inside the `InputTemplate`.

- It takes **some time** for the **EventBridge archive to ingest the events**. Be patient.

- For some reason, the CDK does not provide a way to specify a JSON path for the `detail-type` and `source` parameters in the StepFunctions integration with EventBridge.

  - Luckily, I can use the SDK integrations where I have complete control over parameters.

## Deployment

1. `npm run bootstrap`
1. `npm run deploy`
1. `npm run publish`
1. Wait for the events to be available within the archive
1. `npm run replay`
1. Verify the order of the events – check the CloudWatch log group.
