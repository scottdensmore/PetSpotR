#!/usr/bin/env bash
set -euo pipefail

PUBSUB_HOST="${PUBSUB_EMULATOR_HOST:-localhost:8086}"
FIRESTORE_HOST="${FIRESTORE_EMULATOR_HOST:-localhost:8085}"
PROJECT_ID="${GOOGLE_CLOUD_PROJECT:-petspotr-local}"

echo "==> Initializing PetSpotR emulators for project '${PROJECT_ID}'..."
echo "    Pub/Sub emulator:  ${PUBSUB_HOST}"
echo "    Firestore emulator: ${FIRESTORE_HOST}"

# Wait for Pub/Sub emulator
echo "==> Waiting for Pub/Sub emulator to accept connections at http://${PUBSUB_HOST}/..."
retries=30
until curl -fsS "http://${PUBSUB_HOST}/" >/dev/null 2>&1; do
  retries=$((retries - 1))
  if [ "$retries" -le 0 ]; then
    echo "ERROR: Timed out waiting for Pub/Sub emulator at ${PUBSUB_HOST}" >&2
    exit 1
  fi
  sleep 1
done
echo "    Pub/Sub emulator is ready."

# Wait for Firestore emulator
echo "==> Waiting for Firestore emulator to accept connections at http://${FIRESTORE_HOST}/..."
retries=30
until curl -fsS "http://${FIRESTORE_HOST}/" >/dev/null 2>&1; do
  retries=$((retries - 1))
  if [ "$retries" -le 0 ]; then
    echo "ERROR: Timed out waiting for Firestore emulator at ${FIRESTORE_HOST}" >&2
    exit 1
  fi
  sleep 1
done
echo "    Firestore emulator is ready."

# Canonical topics to initialize
TOPICS=(
  "lostPet"
  "foundPet"
  "matchFound"
  "petStatusChanged"
  "lostPet-dlq"
  "foundPet-dlq"
  "matchFound-dlq"
  "petStatusChanged-dlq"
)

echo "==> Creating canonical Pub/Sub topics..."
for topic in "${TOPICS[@]}"; do
  topic_path="projects/${PROJECT_ID}/topics/${topic}"
  response=$(curl -s -w "\n%{http_code}" -X PUT "http://${PUBSUB_HOST}/v1/${topic_path}")
  http_code=$(echo "$response" | tail -n 1)
  if [ "$http_code" = "200" ] || [ "$http_code" = "409" ]; then
    echo "    Topic ${topic_path} initialized (status: ${http_code})."
  else
    echo "ERROR: Failed to create topic ${topic_path} (status: ${http_code}): $response" >&2
    exit 1
  fi
done

# Push subscriptions: (sub_name topic_name push_endpoint)
declare -a SUBSCRIPTIONS=(
  "lost-pet-matcher-analysis:lostPet:http://pet-matcher:8083/events/lost-pets"
  "found-pet-matcher:foundPet:http://pet-matcher:8083/events/found-pets"
  "match-found-notification-backlog:matchFound:http://notification-service:8084/events/matches"
  "lost-pet-notification:lostPet:http://notification-service:8084/events/lost-pets"
)

echo "==> Creating canonical Pub/Sub push subscriptions..."
for entry in "${SUBSCRIPTIONS[@]}"; do
  IFS=":" read -r sub_name topic_name endpoint_url <<< "$entry"
  # Reassemble URL in case IFS split on protocol colon
  endpoint_url="${entry#*:*:}"
  sub_path="projects/${PROJECT_ID}/subscriptions/${sub_name}"
  topic_path="projects/${PROJECT_ID}/topics/${topic_name}"

  payload=$(cat <<EOF
{
  "topic": "${topic_path}",
  "pushConfig": {
    "pushEndpoint": "${endpoint_url}"
  }
}
EOF
)
  response=$(curl -s -w "\n%{http_code}" -X PUT "http://${PUBSUB_HOST}/v1/${sub_path}"     -H "Content-Type: application/json"     -d "$payload")
  http_code=$(echo "$response" | tail -n 1)
  if [ "$http_code" = "200" ] || [ "$http_code" = "409" ]; then
    echo "    Subscription ${sub_path} -> ${endpoint_url} initialized (status: ${http_code})."
  else
    echo "ERROR: Failed to create subscription ${sub_path} (status: ${http_code}): $response" >&2
    exit 1
  fi
done

echo "==> Emulator initialization completed successfully."
