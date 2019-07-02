#!/bin/bash

set -eo pipefail

if [[ -z "$1" ]] || [[ -z "$2" ]]; then
    echo "usage: $0 service_account_name namespace [outfile]"
    exit 1
fi

SERVICEACCOUNT="$1"
NAMESPACE="$2"
OUT="${3:-/dev/stdout}"
CACERT=$(mktemp)
KUBECONFIG=$(mktemp)

secret=$(kubectl -n "$NAMESPACE" get sa "$SERVICEACCOUNT" -o=jsonpath="{.secrets[0].name}")
token=$(kubectl get secret "$secret" -o=jsonpath="{.data.token}" | base64 -d)
kubectl get secret "$secret" -o=jsonpath="{.data.ca\.crt}" | base64 -d > "$CACERT"

context=$(kubectl config current-context)
cluster_name=$(kubectl config get-contexts "$context" | tail -n1 | xargs | cut -f3 -d" ")
server=$(kubectl config view -o jsonpath="{.clusters[?(@.name == \"${cluster_name}\")].cluster.server}")


kubectl config set-cluster --kubeconfig="$KUBECONFIG" \
        "$cluster_name" \
        --server="$server" \
        --certificate-authority="$CACERT" \
        --embed-certs=true
kubectl config set-credentials --kubeconfig="$KUBECONFIG"  \
        "$SERVICEACCOUNT-$NAMESPACE-$cluster_name" \
        --token="$token"

kubectl config set-context --kubeconfig="$KUBECONFIG" \
        "$SERVICEACCOUNT-$NAMESPACE-$cluster_name" \
        --cluster="$cluster_name" \
        --user="$SERVICEACCOUNT-$NAMESPACE-$cluster_name" \
        --namespace="$NAMESPACE"

kubectl config use-context --kubeconfig="$KUBECONFIG" \
        "$SERVICEACCOUNT-$NAMESPACE-$cluster_name"

cat "$KUBECONFIG" > "$OUT"
rm "$KUBECONFIG" "$CACERT"

