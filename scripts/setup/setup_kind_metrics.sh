#!/bin/bash

NODE=$(kubectl get nodes --show-labels --no-headers -o wide | grep name="knative-control-plane" | awk '{print $6}')
server_exec() { 
	ssh -oStrictHostKeyChecking=no -p 22 $NODE $1;
}
# Install git
server_exec 'sudo apt update'
server_exec 'sudo apt install -y git-all'

# Install tmux
server_exec 'sudo apt install -y tmux'

# Install helm
server_exec 'curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3'
server_exec 'chmod 700 get_helm.sh'
server_exec './get_helm.sh'

# Add prometheus to local helm repository
server_exec 'helm repo add prometheus-community https://prometheus-community.github.io/helm-charts'
server_exec 'helm repo update'

# Config kubectl config
docker exec knative-control-plane sh -c "mkdir -p /home/$(whoami)/.kube"
docker cp ~/.kube/config knative-control-plane:/home/$(whoami)/.kube/config
docker exec knative-control-plane sh -c "echo 'export KUBECONFIG=/home/$(whoami)/.kube/config' >> /home/$(whoami)/.bashrc"
docker exec knative-control-plane sh -c "sudo chown $(whoami):$(whoami) /home/$(whoami)/.kube/config"
docker exec knative-control-plane sh -c "sed -i 's#https://127\.0\.0\.1:[0-9]\{1,\}#https://127.0.0.1:6443#g' /home/$(whoami)/.kube/config"

# Create namespace monitoring to deploy all services in that namespace
server_exec 'kubectl create namespace monitoring'

# Install kube-prometheus stack
release_label="prometheus"
prometheus_chart_version="60.1.0"

scp ./config/prometh_values_kn.yaml $NODE:/home/$(whoami)/prometh_values_kn.yaml

server_exec "helm install \
    -n monitoring $release_label \
    --version $prometheus_chart_version prometheus-community/kube-prometheus-stack \
    -f /home/$(whoami)/prometh_values_kn.yaml"

server_exec 'tmux new -s prometheusd -d'
server_exec 'tmux send -t prometheusd "while true; do kubectl port-forward -n monitoring svc/prometheus-operated 9090; done" ENTER'