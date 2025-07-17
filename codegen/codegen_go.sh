#!bin/bash

set -o nounset
set -o errexit
set -o pipefail


mkdir -p \
  ./build/source/go

INCLUDES="-Iusr/include -Iusr/include/google -Iprotos/key"

# go
echo "Codegen golang"
protoc $INCLUDES \
    --go_opt paths=import \
    --go_out ./build/source/go \
    --go-grpc_opt paths=import \
    --go-grpc_out ./build/source/go \
    --go-grpc_opt require_unimplemented_servers=false \
    $(find ./protos -type f -name "*.proto")

# grpc-gateway
echo "Codegen grpc-gateway"
protoc $INCLUDES \
    --grpc-gateway_out ./build/source/go \
    --grpc-gateway_opt logtostderr=true \
    --grpc-gateway_opt paths=import \
    --grpc-gateway_opt generate_unbound_methods=false \
    $(find ./protos -type f -name "*.proto")

