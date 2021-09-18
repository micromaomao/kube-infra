#!/bin/bash
set -xe

docker build kube-tester -t registry.digitalocean.com/maowtm-images/kube-tester -f kube-tester/Dockerfile
docker push registry.digitalocean.com/maowtm-images/kube-tester
docker build custom-controllers/secret-copy -t registry.digitalocean.com/maowtm-images/secret-copy
docker push registry.digitalocean.com/maowtm-images/secret-copy
