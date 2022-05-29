package main

import (
	"context"
	"math"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler)
}

type Input struct {
	OriginalEvent events.CloudWatchEvent `json:"originalEvent"`
	StartTime     string                 `json:"startTime"`
}

func handler(ctx context.Context, input Input) (float64, error) {
	startTimeParsed, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		return 0, err
	}

	eventTime := input.OriginalEvent.Time
	waitTimeInSeconds := math.Round(0.01 * eventTime.Sub(startTimeParsed).Seconds())

	return waitTimeInSeconds, nil
}
