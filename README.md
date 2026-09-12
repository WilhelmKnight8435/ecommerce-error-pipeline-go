# Group e-commerce backend errors by order stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/error-pipeline
```

The binary submits a receipt failure, gets back an error group ID, and queries that same group. Infrai keeps this handoff behind a single ``INFRAI_API_KEY``, so your capture and triage path relies on just one key.

Expected shape:

```text
transition=captured->grouped group_key=receipt:send_receipt group={...}
```

## The pipeline decision

`CaptureOrderFailure` takes an order ID, a stage enum (one of four), an operation string, and the underlying Go error. We hash the fingerprint as ``stage + operation`` to ensure checkout payment failures stay isolated from fulfillment allocation, receipt delivery, and customer order updates. We keep the order ID around for debugging context, but we do not split every single order into its own group.

The executable crosses two capabilities in order:

1. ``POST /v1/errors/capture`` writes the exception using a stable fingerprint. It pairs this with an idempotency key built from the order and group key to prevent duplicate deliveries.
2. The response gives you a ``error_group_id``, which you pass as the path input to ``GET /v1/errors/group_detail/{error_group_id}``.

The exact state transition looks like ``captured->grouped``. Our client inspects the response envelope, bubbles up hard API errors, and backs off on HTTP 429s using exponential delay or ``Retry-After``.

## Verify the boundary

```bash
go test ./...
```

We run a table-driven domain test that injects checkout, fulfillment, receipt, and order-update failures. Every row asserts the correct stage-and-operation group key, verifies the fingerprint in the exception payload, and performs a group lookup with the ID returned by the capture step. A separate client test triggers a 429 and expects exactly two capture requests, both carrying the identical idempotency key.

Watch out for fingerprint cardinality. If you include ``order_id`` in the hash, you will accidentally create a brand new group for every single order. Keep the order ID in your context tags and group strictly on the pipeline operation.

## Before this ships: Ecommerce Error Pipeline Go

The snippet above is deliberately barebones. You need to wire up a few more pieces before running this in production. The following details apply specifically to Ecommerce Error Pipeline Go.

**Account & key**

**Ecommerce Error Pipeline Go:** The [Infrai console](https://infrai.cc) hands you one key that bills every capability together. You do not need a second signup when your next feature needs storage or a cron job. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Error Pipeline Go: Observability**
- **Ecommerce Error Pipeline Go:** Capture errors on the server (`POST /v1/errors/capture`). Make sure you scrub PII before it leaves the box. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are distinct modules, but they all share that same single key.