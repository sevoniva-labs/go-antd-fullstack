#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${FORGE_ROCKETMQ_IMAGE:?set FORGE_ROCKETMQ_IMAGE to an approved immutable RocketMQ image digest}"
: "${FORGE_ROCKETMQ_ACCESS_KEY:?set FORGE_ROCKETMQ_ACCESS_KEY in the local environment}"
: "${FORGE_ROCKETMQ_SECRET_KEY:?set FORGE_ROCKETMQ_SECRET_KEY in the local environment}"

if [[ "$FORGE_ROCKETMQ_IMAGE" != *@sha256:* ]]; then
  echo "FORGE_ROCKETMQ_IMAGE must use an immutable sha256 digest" >&2
  exit 1
fi

COMPOSE_CMD=${FORGE_COMPOSE_CMD:-docker compose}
read -r -a COMPOSE <<<"$COMPOSE_CMD"
PROJECT=${FORGE_ROCKETMQ_PROJECT:-forge-rocketmq-contract-$$}
COMPOSE_FILE=${FORGE_ROCKETMQ_COMPOSE_FILE:-deploy/compose/rocketmq-dev.yaml}
RUNTIME_ENDPOINT=${FORGE_ROCKETMQ_RUNTIME_ENDPOINT:-127.0.0.1:8081}
NAMESPACE=${FORGE_ROCKETMQ_NAMESPACE-}
TOPIC=${FORGE_ROCKETMQ_TOPIC:-forge-contract-${PROJECT##*-}}
GROUP=${FORGE_ROCKETMQ_GROUP:-forge-contract-group-${PROJECT##*-}}
RUNTIME_GOARCH=${FORGE_ROCKETMQ_RUNTIME_GOARCH:-$(go env GOARCH)}
EVIDENCE_FILE=${FORGE_ROCKETMQ_EVIDENCE_FILE:-}

export ROCKETMQ_IMAGE="$FORGE_ROCKETMQ_IMAGE"

compose() {
  "${COMPOSE[@]}" -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

source_file="$ROOT/rocketmq-contract.${PROJECT##*-}.tmp.go"
binary_file="$ROOT/rocketmq-contract.${PROJECT##*-}.tmp.bin"

cleanup() {
  rm -f "$source_file" "$binary_file"
  compose down -v >/dev/null 2>&1 || true
}

trap cleanup EXIT

compose config >/dev/null
compose up -d >/dev/null 2>&1

broker_ready=false
for _ in $(seq 1 90); do
  if compose exec -T rocketmq sh mqadmin clusterList -n rocketmq-namesrv:9876 >/dev/null 2>&1; then
    broker_ready=true
    break
  fi
  sleep 1
done

if [[ "$broker_ready" != true ]]; then
  printf 'RocketMQ contract failed: broker did not become ready\n' >&2
  compose ps >&2 || true
  exit 1
fi

if ! compose exec -T rocketmq sh mqadmin updateTopic -n rocketmq-namesrv:9876 -b 127.0.0.1:10911 -t "$TOPIC" >/dev/null 2>&1; then
  printf 'RocketMQ contract failed: could not create the unique test topic\n' >&2
  exit 1
fi

route_ready=false
for _ in $(seq 1 60); do
  if compose exec -T rocketmq sh mqadmin topicRoute -n rocketmq-namesrv:9876 -t "$TOPIC" >/dev/null 2>&1; then
    route_ready=true
    break
  fi
  sleep 1
done

if [[ "$route_ready" != true ]]; then
  printf 'RocketMQ contract failed: nameserver route for the test topic did not become visible\n' >&2
  exit 1
fi

cat > "$source_file" <<'GO'
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("missing " + name)
	}
	return value
}

func localClientFactory(config *rmq.Config, _ ...rmq.ClientOption) (rmq.Client, error) {
	return rmq.NewClient(config, rmq.WithClientConnFunc(func(endpoint string, _ ...rmq.ConnOption) (rmq.ClientConn, error) {
		conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		return &localClientConn{conn: conn}, nil
	}))
}

type localClientConn struct {
	conn *grpc.ClientConn
}

func (c *localClientConn) Conn() *grpc.ClientConn { return c.conn }
func (c *localClientConn) Close() error           { return c.conn.Close() }

func main() {
	endpoint := required("FORGE_ROCKETMQ_ENDPOINT")
	namespace := os.Getenv("FORGE_ROCKETMQ_NAMESPACE")
	topic := required("FORGE_ROCKETMQ_TOPIC")
	group := required("FORGE_ROCKETMQ_GROUP")
	accessKey := required("FORGE_ROCKETMQ_ACCESS_KEY")
	secretKey := required("FORGE_ROCKETMQ_SECRET_KEY")
	body := []byte(fmt.Sprintf("rocketmq-contract-%d", time.Now().UnixNano()))

	producer, err := rmq.NewProducer(&rmq.Config{
		Endpoint: endpoint, NameSpace: namespace, ConsumerGroup: group,
		Credentials: &credentials.SessionCredentials{AccessKey: accessKey, AccessSecret: secretKey},
	}, rmq.WithTopics(topic), rmq.WithClientFunc(localClientFactory))
	if err != nil {
		panic(fmt.Errorf("create producer: %w", err))
	}
	if err := producer.Start(); err != nil {
		panic(fmt.Errorf("start producer: %w", err))
	}
	defer producer.GracefulStop()

	consumer, err := rmq.NewSimpleConsumer(&rmq.Config{
		Endpoint: endpoint, NameSpace: namespace, ConsumerGroup: group,
		Credentials: &credentials.SessionCredentials{AccessKey: accessKey, AccessSecret: secretKey},
	}, rmq.WithSimpleSubscriptionExpressions(map[string]*rmq.FilterExpression{
		topic: rmq.NewFilterExpression("*"),
	}), rmq.WithSimpleAwaitDuration(5*time.Second), rmq.WithClientFuncForSimpleConsumer(localClientFactory))
	if err != nil {
		panic(fmt.Errorf("create consumer: %w", err))
	}
	if err := consumer.Start(); err != nil {
		panic(fmt.Errorf("start consumer: %w", err))
	}
	defer consumer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	receipts, err := producer.Send(ctx, &rmq.Message{Topic: topic, Body: body})
	cancel()
	if err != nil {
		panic(fmt.Errorf("send message: %w", err))
	}
	if len(receipts) == 0 || receipts[0].MessageID == "" {
		panic("send message returned no receipt")
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		views, receiveErr := consumer.Receive(ctx, 1, 30*time.Second)
		cancel()
		if receiveErr != nil {
			continue
		}
		for _, view := range views {
			if string(view.GetBody()) != string(body) {
				continue
			}
			if err := consumer.Ack(context.Background(), view); err != nil {
				panic(fmt.Errorf("ack message: %w", err))
			}
			fmt.Println("RocketMQ SDK produce-consume contract passed")
			return
		}
	}

	panic("timed out waiting for the produced message")
}
GO

FORGE_ROCKETMQ_ENDPOINT="$RUNTIME_ENDPOINT" \
FORGE_ROCKETMQ_NAMESPACE="$NAMESPACE" \
FORGE_ROCKETMQ_TOPIC="$TOPIC" \
FORGE_ROCKETMQ_GROUP="$GROUP" \
FORGE_ROCKETMQ_ACCESS_KEY="$FORGE_ROCKETMQ_ACCESS_KEY" \
FORGE_ROCKETMQ_SECRET_KEY="$FORGE_ROCKETMQ_SECRET_KEY" \
GOPROXY="${GOPROXY:-https://goproxy.cn}" \
GOSUMDB="${GOSUMDB:-sum.golang.org https://goproxy.cn/sumdb/sum.golang.org}" \
GOOS=linux GOARCH="$RUNTIME_GOARCH" CGO_ENABLED=0 go build -o "$binary_file" "$source_file"

if ! cat "$binary_file" | compose exec -T \
  -e "FORGE_ROCKETMQ_ENDPOINT=$RUNTIME_ENDPOINT" \
  -e "FORGE_ROCKETMQ_NAMESPACE=$NAMESPACE" \
  -e "FORGE_ROCKETMQ_TOPIC=$TOPIC" \
  -e "FORGE_ROCKETMQ_GROUP=$GROUP" \
  -e "FORGE_ROCKETMQ_ACCESS_KEY=$FORGE_ROCKETMQ_ACCESS_KEY" \
  -e "FORGE_ROCKETMQ_SECRET_KEY=$FORGE_ROCKETMQ_SECRET_KEY" \
  rocketmq sh -c 'cat > /tmp/forge-rocketmq-contract && chmod 700 /tmp/forge-rocketmq-contract && exec /tmp/forge-rocketmq-contract'; then
  printf 'RocketMQ contract failed: SDK produce-consume helper failed\n' >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  python3 - "$EVIDENCE_FILE" "$PROJECT" "$FORGE_ROCKETMQ_IMAGE" "$(git rev-parse HEAD)" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

path, project, image, commit = sys.argv[1:]
payload = {
    "kind": "rocketmq-runtime-contract",
    "status": "passed",
    "project": project,
    "rocketmq_image": image,
    "source_commit": commit,
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "checks": [
        "nameserver-broker-ready",
        "unique-topic-created",
        "nameserver-topic-route-visible",
        "sdk-produce-consume-ack",
    ],
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

printf 'RocketMQ runtime contract passed: image=%s\n' "$FORGE_ROCKETMQ_IMAGE"
