#!/usr/bin/env bash
set -euo pipefail

RUN_DIR=$(pwd)

TEMP_USERNAME=siderolabs

# if PUSH_TAG is empty, set it to be $IMAGER_TAG-agent-$BUILD_TAG, e.g., v1.9.0-agent-8f45b43
if [ -z "$PUSH_TAG" ]; then
  PUSH_TAG="$IMAGER_TAG-agent-$BUILD_TAG"
fi

echo "Using PUSH_TAG=$PUSH_TAG"

# build and push the agent image

make image-talos-metal-agent PUSH=true REGISTRY="$TEMP_REGISTRY" USERNAME="$TEMP_USERNAME" TAG="$PUSH_TAG" PLATFORM=linux/amd64,linux/arm64

TEMP_DIR=$(mktemp -d -t agent-boot-assets-XXXXX)

function cleanup() {
  cd "${RUN_DIR}"
  rm -rf "$TEMP_DIR"
}

trap cleanup EXIT SIGINT

echo "Building in $TEMP_DIR"

cd "$TEMP_DIR"

# build and push the extension using the agent image we built above

git clone "$EXTENSIONS_REPO" extensions
cd extensions
git -c advice.detachedHead=false checkout "$EXTENSIONS_REF"

yq e -i ".IMAGE_PREFIX = \"$TEMP_REGISTRY/$TEMP_USERNAME\"" "$EXTENSIONS_PATH/vars.yaml"
yq e -i ".VERSION = \"$PUSH_TAG\"" "$EXTENSIONS_PATH/vars.yaml"

make metal-agent PUSH=true REGISTRY="$TEMP_REGISTRY" USERNAME="$TEMP_USERNAME" TAG="$PUSH_TAG"

EXTENSION_IMAGE="$TEMP_REGISTRY/$TEMP_USERNAME/metal-agent:$PUSH_TAG"
IMAGER_IMAGE="$IMAGER_REGISTRY_AND_USERNAME/imager:$IMAGER_TAG"

mapfile -t FIRMWARE_EXTENSIONS <<'EOF'
siderolabs/amd-ucode
siderolabs/amdgpu-firmware
siderolabs/bnx2-bnx2x
siderolabs/chelsio-firmware
siderolabs/i915-ucode
siderolabs/intel-ice-firmware
siderolabs/intel-ucode
siderolabs/qlogic-firmware
siderolabs/realtek-firmware
EOF

# build talos boot artifacts with the extension we built above using imager, for both amd64 and arm64

function filter_firmware_extensions() {
  # Read input from stdin and process each line
  while IFS= read -r line; do
    # Check if any directory name matches in the line
    for dir in "${FIRMWARE_EXTENSIONS[@]}"; do
      if [[ $line == *"$dir"* ]]; then
        echo "$line"
        break
      fi
    done
  done
}

function build_image_profile() {
  local arch=$1
  local kind=$2
  local extensions

  # prepare extensions list with proper indentation
  extensions=$(crane export "$EXTENSION_DIGESTS_IMAGE:$IMAGER_TAG" |
    tar x -O image-digests |
    filter_firmware_extensions |
    sed 's/^/    - imageRef: /')

  cat <<EOF
arch: $arch
platform: metal
version: $IMAGER_TAG
input:
  kernel:
    path: /usr/install/$arch/vmlinuz
  initramfs:
    path: /usr/install/$arch/initramfs.xz
  systemExtensions:
$extensions
    - imageRef: $EXTENSION_IMAGE
output:
  kind: $kind
  outFormat: raw
EOF
}

ASSETS_DIR="$TEMP_DIR/assets"

mkdir -p "$ASSETS_DIR"

SCRIPT_DIR="$RUN_DIR/hack/boot-assets"

function build_artifacts() {
  local arch=$1

  build_image_profile "$arch" "initramfs" | docker run --rm -i --network=host --privileged -v "$ASSETS_DIR:/out" "$IMAGER_IMAGE" -
  build_image_profile "$arch" "kernel" | docker run --rm -i --network=host --privileged -v "$ASSETS_DIR:/out" "$IMAGER_IMAGE" -
  build_image_profile "$arch" "cmdline" | docker run --rm -i --network=host --privileged -v "$ASSETS_DIR:/out" "$IMAGER_IMAGE" -
}

build_artifacts "amd64"
build_artifacts "arm64"

# build and push the final image containing all the artifacts

FINAL_IMAGE="$OUTPUT_REGISTRY_AND_USERNAME/talos-metal-agent-boot-assets:$PUSH_TAG"

cd "$TEMP_DIR"

cp "$SCRIPT_DIR/Dockerfile" .

docker build -t "$FINAL_IMAGE" .

echo "Built image: $FINAL_IMAGE"
