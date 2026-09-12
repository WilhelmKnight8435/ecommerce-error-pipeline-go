package errorpipeline

import (
	"context"
	"encoding/json"
	"fmt"
)

type OrderStage string

const (
	CheckoutStage    OrderStage = "checkout"
	FulfillmentStage OrderStage = "fulfillment"
	ReceiptStage     OrderStage = "receipt"
	OrderUpdateStage OrderStage = "order_update"
)

type OrderFailure struct {
	OrderID   string
	Stage     OrderStage
	Operation string
	Err       error
}

type CaptureResult struct {
	Event      json.RawMessage
	Group      json.RawMessage
	GroupKey   string
	Transition string
}

type ErrorAPI interface {
	Capture(context.Context, map[string]any, string) (json.RawMessage, error)
	GroupDetail(context.Context, string) (json.RawMessage, error)
}

func CaptureOrderFailure(ctx context.Context, api ErrorAPI, failure OrderFailure) (CaptureResult, error) {
	groupKey, err := groupKeyFor(failure)
	if err != nil {
		return CaptureResult{}, err
	}
	exception := map[string]any{
		"message":     failure.Err.Error(),
		"fingerprint": []string{string(failure.Stage), failure.Operation},
		"context": map[string]string{
			"order_id":  failure.OrderID,
			"stage":     string(failure.Stage),
			"operation": failure.Operation,
		},
	}
	event, err := api.Capture(ctx, exception, "order-error:"+failure.OrderID+":"+groupKey)
	if err != nil {
		return CaptureResult{}, err
	}

	var captured struct {
		ErrorGroupID string `json:"error_group_id"`
	}
	if err := json.Unmarshal(event, &captured); err != nil {
		return CaptureResult{}, fmt.Errorf("decode capture data: %w", err)
	}
	if captured.ErrorGroupID == "" {
		return CaptureResult{}, fmt.Errorf("capture data missing error_group_id")
	}
	group, err := api.GroupDetail(ctx, captured.ErrorGroupID)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{Event: event, Group: group, GroupKey: groupKey, Transition: "captured->grouped"}, nil
}

func groupKeyFor(failure OrderFailure) (string, error) {
	if failure.OrderID == "" || failure.Operation == "" || failure.Err == nil {
		return "", fmt.Errorf("order id, operation, and error are required")
	}
	switch failure.Stage {
	case CheckoutStage, FulfillmentStage, ReceiptStage, OrderUpdateStage:
		return string(failure.Stage) + ":" + failure.Operation, nil
	default:
		return "", fmt.Errorf("unknown order stage %q", failure.Stage)
	}
}
