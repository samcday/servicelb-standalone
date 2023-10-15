# servicelb-standalone

This is a hacked together proof-of-concept that makes the [k3s Service Load Balancer](https://docs.k3s.io/networking#service-load-balancer) available as a standalone controller that can run in any Kubernetes cluster.

This project is not packaged for general use, as it is only intended to demonstrate how straightforward it is to extract the ServiceLB functionality from the k3s monolithic codebase.

## Testing

[Skaffold](https://skaffold.dev) is required. Write access to a container registry is also required.

```
export SKAFFOLD_DEFAULT_REPO=<https://a-container-registry-you-have-access.to>
skaffold dev
```

Once the controller is running, you can create a LoadBalancer service as per the standard ServiceLB documentation.
