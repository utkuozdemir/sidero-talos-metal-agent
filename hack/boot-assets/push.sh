#!/usr/bin/env bash
set -euo pipefail

# if PUSH_TAG is empty, set it to be $IMAGER_TAG-agent-$BUILD_TAG, e.g., v1.9.0-agent-8f45b43
if [ -z "$PUSH_TAG" ]; then
  PUSH_TAG="$IMAGER_TAG-agent-$BUILD_TAG"
fi

# build and push the final image containing all the artifacts

FINAL_IMAGE="$OUTPUT_REGISTRY_AND_USERNAME/talos-metal-agent-boot-assets:$PUSH_TAG"

docker push "$FINAL_IMAGE"

echo "Pushed image: $FINAL_IMAGE"
