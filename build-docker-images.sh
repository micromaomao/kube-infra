#!/bin/bash
set -xe

docker build kube-tester -t ghcr.io/micromaomao/kube-tester -f kube-tester/Dockerfile
docker push ghcr.io/micromaomao/kube-tester
docker build custom-controllers/secret-copy -t ghcr.io/micromaomao/secret-copy
docker push ghcr.io/micromaomao/secret-copy
