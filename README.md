# KubeChaos Operator

This project implements a Kubernetes operator, `kubechaos-operator`, to introduce chaos engineering capabilities into your cluster. It uses a custom resource called `ChaosExperiment` to define and manage various chaos experiments.

## Features

- **ChaosExperiment CRD**: Define chaos experiments using a Custom Resource Definition.
- **Pod Kill Attack**: Currently supports `pod-kill` to randomly delete pods matching a label selector.
- **Experiment Phases**: Tracks experiment lifecycle with `Pending`, `Running`, `Completed`, and `Failed` phases.
- **Recurring Experiments**: Supports one-shot and recurring experiment modes.
- **Kubernetes Events**: Emits events for key actions like experiment start, pod kills, and completion.

## Prerequisites

- Go (1.20+)
- Docker (for building and pushing images)
- kubectl
- Access to a Kubernetes cluster
- [Kubebuilder](https://book.kubebuilder.io/quick-start.html)

## Getting Started

Follow these steps to deploy and run the `kubechaos-operator` in your Kubernetes cluster.

### 1. Install CRDs

First, install the Custom Resource Definitions (CRDs) for `ChaosExperiment` into your cluster.

```bash
make install
```

This command will create the `chaosexperiments.chaos.shanto.dev` CRD. You can verify its installation:

```bash
kubectl get crds | grep chaosexperiments
```

### 2. Run the Controller Manager

You can run the controller manager locally against your Kubernetes cluster. This is useful for development and testing. Ensure your `KUBECONFIG` environment variable is set correctly to point to your cluster.

```bash
make run
```

This command will start the operator in your local environment, connecting to the configured Kubernetes cluster. It will watch for `ChaosExperiment` resources and reconcile them.

### 3. Apply a Sample ChaosExperiment

We have provided a sample `ChaosExperiment` manifest that targets pods with the label `app=nginx` in the `demo` namespace.

Before applying the sample, ensure you have a namespace named `demo` and some pods with the label `app=nginx` running in it.

```bash
# Create the demo namespace if it doesn't exist
kubectl create namespace demo

# Example: Deploy a simple nginx application for testing
kubectl create deployment nginx --image=nginx -n demo
kubectl scale deployment/nginx --replicas=3 -n demo
```

Now, apply the sample `ChaosExperiment`:

```bash
kubectl apply -f config/samples/chaos_v1alpha1_chaosexperiment_pod_kill.yaml
```

This experiment is configured for a `pod-kill` attack, targeting `app=nginx` pods in the `demo` namespace, with a `duration` of 60 seconds and `recurring` mode.

### 4. Observe the Experiment

You can observe the status of your `ChaosExperiment` and the effects on your pods:

```bash
kubectl get chaosexperiment pod-kill-nginx-demo -o yaml
kubectl get events -n demo --field-selector involvedObject.name=pod-kill-nginx-demo
kubectl get pods -n demo -w
```

You should see pods being randomly deleted and new ones created (if your deployment has a replica set controller) and events emitted by the operator.

## Building and Deploying to the Cluster

To build the operator image and deploy it directly into your cluster, you can use the following commands:

```bash
# Set your image repository (e.g., your-dockerhub-username/kubechaos-operator)
IMG="your-registry/kubechaos-operator:latest" make docker-build
IMG="your-registry/kubechaos-operator:latest" make docker-push
IMG="your-registry/kubechaos-operator:latest" make deploy
```

Remember to replace `your-registry` with your actual image registry path.