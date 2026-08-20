#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/kafka-streaming-dev.yaml"
KAFKA_IMAGE="${FORGE_KAFKA_IMAGE:?FORGE_KAFKA_IMAGE must be set to an immutable image digest}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
RUNTIME_GOARCH="${FORGE_KAFKA_GOARCH:-$(go env GOARCH)}"
EVIDENCE_FILE="${FORGE_KAFKA_EVIDENCE_FILE:-}"
PROJECT="${FORGE_KAFKA_COMPOSE_PROJECT:-forge-kafka-contract}"
TOPIC="forge-contract-$(date +%s)-$$"
GROUP="forge-contract-group-$(date +%s)-$$"
SOURCE_FILE="$(mktemp "$ROOT/kafka-contract-source.XXXXXX").go"
BINARY_FILE="$(mktemp "$ROOT/kafka-contract-binary.XXXXXX")"

if [[ ! "$KAFKA_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_KAFKA_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi

compose() {
  # shellcheck disable=SC2086
  COMPOSE_PROJECT_NAME="$PROJECT" KAFKA_IMAGE="$KAFKA_IMAGE" \
    sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

cleanup() {
  compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$SOURCE_FILE" "$BINARY_FILE"
}
trap cleanup EXIT

cat >"$SOURCE_FILE" <<EOF
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	const broker = "kafka:9092"
	topic := "${TOPIC}"
	group := "${GROUP}"
	payload := []byte("forge-kafka-contract")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.ClientID("forge-kafka-contract"),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(group),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		panic(fmt.Errorf("kafka ping: %w", err))
	}
	if err := client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte("contract"), Value: payload}).FirstErr(); err != nil {
		panic(fmt.Errorf("kafka produce: %w", err))
	}

	for ctx.Err() == nil {
		fetches := client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) != 0 {
			panic(fmt.Errorf("kafka consume: %v", errs[0].Err))
		}
		matched := false
		fetches.EachRecord(func(record *kgo.Record) {
			if record.Topic == topic && string(record.Value) == string(payload) {
				matched = true
			}
		})
		if matched {
			fmt.Println("Kafka franz-go produce-consume contract passed")
			return
		}
	}
	panic(errors.New("kafka consume timed out"))
}
EOF

compose -f "$COMPOSE_FILE" up -d
for _ in $(seq 1 40); do
  if compose -f "$COMPOSE_FILE" exec -T kafka /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server localhost:9092 --list >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
compose -f "$COMPOSE_FILE" exec -T kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 --create --if-not-exists --topic "$TOPIC" \
  --partitions 1 --replication-factor 1 >/dev/null

GOPROXY=https://goproxy.cn GOSUMDB='sum.golang.org https://goproxy.cn/sumdb/sum.golang.org' \
  GOOS=linux GOARCH="$RUNTIME_GOARCH" CGO_ENABLED=0 \
  go build -o "$BINARY_FILE" "$SOURCE_FILE"
cat "$BINARY_FILE" | compose -f "$COMPOSE_FILE" exec -T kafka sh -c \
  'cat > /tmp/forge-kafka-contract && chmod 700 /tmp/forge-kafka-contract && exec /tmp/forge-kafka-contract'

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  cat >"$EVIDENCE_FILE" <<EOF
{
  "image": "${KAFKA_IMAGE}",
  "topic": "${TOPIC}",
  "streaming_provider": "kafka",
  "kafka_mode": "single-node-kraft-development",
  "franz_go_produce_consume": true
}
EOF
fi

echo "Kafka runtime contract passed: image=$KAFKA_IMAGE"
