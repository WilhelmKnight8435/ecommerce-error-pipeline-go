package main

import (
	"context"
	"fmt"
	"log"
	"os"

	errorpipeline "github.com/example/ecommerce-error-pipeline"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := &errorpipeline.Client{APIKey: key}
	result, err := errorpipeline.CaptureOrderFailure(context.Background(), client, errorpipeline.OrderFailure{
		OrderID:   "ord-1042",
		Stage:     errorpipeline.ReceiptStage,
		Operation: "send_receipt",
		Err:       fmt.Errorf("receipt renderer rejected order currency"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("transition=%s group_key=%s group=%s\n", result.Transition, result.GroupKey, result.Group)
}
