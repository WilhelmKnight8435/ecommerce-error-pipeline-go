package errorpipeline

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

type recordingAPI struct {
	payload map[string]any
	key     string
	groupID string
}

func (r *recordingAPI) Capture(_ context.Context, payload map[string]any, key string) (json.RawMessage, error) {
	r.payload, r.key = payload, key
	return json.RawMessage(`{"event_id":"evt-7","error_group_id":"grp-4"}`), nil
}

func (r *recordingAPI) GroupDetail(_ context.Context, groupID string) (json.RawMessage, error) {
	r.groupID = groupID
	return json.RawMessage(`{"error_group_id":"grp-4","event_count":3}`), nil
}

func TestCaptureOrderFailureGroupsByStageAndOperation(t *testing.T) {
	tests := []struct {
		name      string
		stage     OrderStage
		operation string
		wantKey   string
	}{
		{"checkout payment", CheckoutStage, "authorize_payment", "checkout:authorize_payment"},
		{"fulfillment allocation", FulfillmentStage, "allocate_stock", "fulfillment:allocate_stock"},
		{"receipt delivery", ReceiptStage, "send_receipt", "receipt:send_receipt"},
		{"customer update", OrderUpdateStage, "publish_status", "order_update:publish_status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &recordingAPI{}
			result, err := CaptureOrderFailure(context.Background(), api, OrderFailure{
				OrderID: "ord-1042", Stage: tt.stage, Operation: tt.operation, Err: errors.New("dependency rejected record"),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.GroupKey != tt.wantKey || result.Transition != "captured->grouped" {
				t.Fatalf("result = %#v", result)
			}
			wantFingerprint := []string{string(tt.stage), tt.operation}
			if !reflect.DeepEqual(api.payload["fingerprint"], wantFingerprint) {
				t.Fatalf("fingerprint = %#v, want %#v", api.payload["fingerprint"], wantFingerprint)
			}
			if api.key != "order-error:ord-1042:"+tt.wantKey || api.groupID != "grp-4" {
				t.Fatalf("handoff key=%q group=%q", api.key, api.groupID)
			}
		})
	}
}
