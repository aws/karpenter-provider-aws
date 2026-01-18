#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

cd $(dirname $0)/../..

printf "🛠️ Building test infra image..."
TEST_IMAGE=$(docker build -q -f test/e2e/infra/Dockerfile .)
echo "done! Test image: $TEST_IMAGE"

FAILED="false"

for CASE_DIR in $(ls -d test/e2e/cases/*); do
  CASE_NAME=$(basename $CASE_DIR)
  printf "🧪 Testing $CASE_NAME..."
  CONTAINER_ID=$(docker run \
    -d \
    --rm \
    --privileged \
    -v $PWD/$CASE_DIR:/test-case \
    $TEST_IMAGE)
  LOG_FILE=$(mktemp)
  if docker exec $CONTAINER_ID bash -c "cd /test-case && ./run.sh" > $LOG_FILE 2>&1; then
    echo "passed! ✅"
  else
    echo "failed! ❌"
    cat $LOG_FILE
    FAILED="true"
  fi
  docker kill $CONTAINER_ID > /dev/null 2>&1
done

if [ "${FAILED}" = "true" ]; then
  echo "❌ Some tests failed!"
  exit 1
else
  echo "✅ All tests passed!"
  exit 0
fi
