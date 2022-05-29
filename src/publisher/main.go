package main

import (
	"app/env"
	"context"
	"fmt"
	"os/signal"
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

	fmt.Println("Sending events")

	err := run(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("Events sent")
}

func run(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	client := eventbridge.NewFromConfig(cfg)

	busName, err := env.Get(env.EVENT_BUS_NAME)
	if err != nil {
		return fmt.Errorf("failed to get event bus archive name: %w", err)
	}

	out, err := client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: makeEvents(10, busName),
	})
	if err != nil {
		return err
	}

	if out.FailedEntryCount != 0 {
		return fmt.Errorf("failed to put events: %v", out.Entries)
	}

	return nil
}

func makeEvents(amount int, eventBusName string) []eventbridgetypes.PutEventsRequestEntry {
	var events []eventbridgetypes.PutEventsRequestEntry
	for i := 0; i < amount; i++ {
		now := time.Now().UTC()
		events = append(events, eventbridgetypes.PutEventsRequestEntry{
			DetailType:   aws.String("test-event"),
			Detail:       aws.String(fmt.Sprintf(`{"id": "%s"}`, now.Format(time.RFC3339Nano))),
			EventBusName: aws.String(eventBusName),
			Source:       aws.String("eb-test-app"),
			Time:         aws.Time(now),
		})
	}
	return events
}
