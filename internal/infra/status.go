package infra

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

const (
	waitInterval      = 2 * time.Second
	resourceTypeStack = "AWS::CloudFormation::Stack"
	statInProgress    = "_IN_PROGRESS"
)

type statusWaiter struct {
	cfn             *cloudformation.Client
	stackID         string
	lastEventID     string
	desiredStatuses []types.StackStatus
	log             *log.Logger
}

func (f Fetcher) wait(ctx context.Context, stackID string, statuses ...types.StackStatus) error {
	w := statusWaiter{
		cfn:             f.cfn,
		stackID:         stackID,
		desiredStatuses: statuses,
		log:             f.log,
	}

	return w.wait(ctx)
}

func (w *statusWaiter) wait(ctx context.Context) error {
	status, err := w.getStatus(ctx)
	if err != nil {
		return err
	}

	if inProgress(status) {
		for inProgress(status) {
			if err := w.logEvents(ctx); err != nil {
				return err
			}

			time.Sleep(waitInterval)

			status, err = w.getStatus(ctx)
			if err != nil {
				return err
			}
		}

		if err := w.logEvents(ctx); err != nil {
			return err
		}
	}

	if slices.Contains(w.desiredStatuses, status) {
		return nil
	}

	return fmt.Errorf("Stack %q finished with unexpected status %s", w.stackID, status)
}

func (w *statusWaiter) getStatus(ctx context.Context) (types.StackStatus, error) {
	resp, err := w.cfn.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &w.stackID,
	})
	if err != nil {
		return "", err
	}

	return resp.Stacks[0].StackStatus, nil
}

func (w *statusWaiter) logEvents(ctx context.Context) error {
	events, err := w.listNewEvents(ctx)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	w.lastEventID = *events[0].EventId

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		timestamp := e.Timestamp.Format(time.RFC3339)
		w.log.Printf("%s: [%s] %s | %s", timestamp, e.ResourceStatus, *e.ResourceType, *e.LogicalResourceId)
	}

	return nil
}

func (w *statusWaiter) listNewEvents(ctx context.Context) ([]types.StackEvent, error) {
	paginator := cloudformation.NewDescribeStackEventsPaginator(w.cfn, &cloudformation.DescribeStackEventsInput{
		StackName: &w.stackID,
	})
	var events []types.StackEvent

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, event := range page.StackEvents {
			if *event.EventId == w.lastEventID {
				return events, nil
			}

			events = append(events, event)

			if w.lastEventID == "" &&
				*event.LogicalResourceId == *event.StackName && *event.ResourceType == resourceTypeStack {
				return events, nil
			}
		}
	}

	return events, nil
}

func inProgress[T ~string](status T) bool {
	return strings.HasSuffix(string(status), statInProgress)
}
