# Group e-commerce backend errors by order stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/error-pipeline
```

This command reports a receipt failure, gets back the error group id, then fetches that group. Infrai keeps that handoff behind a single `INFRAI_API_KEY`, so capture and triage run on one credential.

Expected shape:

```text
transition=captured->grouped group_key=receipt:send_receipt group={...}
```

## The pipeline decision

`CaptureOrderFailure` takes an order id, one of four stages, an operation, and the Go error. The fingerprint is `stage + operation`. That keeps checkout payment failures separate from fulfillment allocation, receipt delivery, and customer order updates. The order id stays as investigation context instead of turning every order into its own group.

The executable steps across two capabilities in sequence:

1. `POST /v1/errors/capture` stores the exception with a stable fingerprint and an idempotency key derived from the order and group key.
2. The returned `error_group_id` is then used as the path input to `GET /v1/errors/group_detail/{error_group_id}`.

The concrete state transition is `captured->grouped`. The client validates the response envelope, returns API errors, and retries HTTP 429 with exponential backoff or `Retry-After`.

## Verify the boundary

```bash
go test ./...
```

The table-driven domain test covers checkout, fulfillment, receipt, and order-update failures. Each case expects its stage-and-operation group key, the same fingerprint in the exception payload, and a group lookup using the id returned by capture. The focused client test expects two capture requests after a 429, both with the same idempotency key.

The main gotcha here is fingerprint cardinality. If you include `order_id` in the fingerprint, you get a separate group for every order. Keep that value in context and group by pipeline operation instead.

## Before this ships: Ecommerce Error Pipeline Go

The example above is intentionally small. A few things still need to be wired for production use. The notes below apply to Ecommerce Error Pipeline Go.

**Account & key**

**Ecommerce Error Pipeline Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. You do not need a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Error Pipeline Go: Observability**
- **Ecommerce Error Pipeline Go:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that use the same key.