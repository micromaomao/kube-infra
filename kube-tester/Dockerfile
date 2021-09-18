FROM debian:latest

RUN apt update -y && apt install curl -y && \
    curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/bin/kubectl && \
    chmod +x /usr/bin/kubectl && \
    echo 'alias k=kubectl' > /root/.bashrc
ENTRYPOINT [ "bash" ]
