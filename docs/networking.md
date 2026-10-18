# Networking

Tasks do not get a Kubernetes Service or Ingress of their own. Every request to a task goes through Agent Substrate's **atenet router**, the `atenet-router` Service in the `ate-system` namespace. The router reads a single header, `ate-target-actor`, resolves the actor to the worker it is running on, resumes it first if it was suspended, and proxies the request there. `Host` and `:authority` are left alone for your application; the header alone selects the target.

The header value is `<atespace>/<task>`. AX always names a task's actor after the task, so `default/task123` reaches the task `task123` in the `default` atespace.

## From inside the cluster

Use the Service DNS name and add the header. This is exactly how AX polls a task's readiness.

```bash
curl -H "ate-target-actor: default/task123" \
  http://atenet-router.ate-system.svc.cluster.local/readyz
```

## From your machine

Port-forward the router, then talk to it the same way.

```bash
kubectl -n ate-system port-forward svc/atenet-router 8001:80
curl -H "ate-target-actor: default/task123" http://localhost:8001/readyz
```

## gRPC request routing

Send the header as outgoing metadata under the lowercase key. This is what `ax ssh` does to reach the guest services.

```go
ctx = metadata.AppendToOutgoingContext(ctx, "ate-target-actor", "default/task123")
resp, err := client.SomeMethod(ctx, req)
```

## Egress

A task has no network access until its actor has an egress policy on Substrate. AX writes that policy from `spec.egress` of the Task before the actor first runs, so a task names what it may reach in its own manifest (see [Writing manifests](manifests.md#network-access)):

```yaml
spec:
  egress:
    rules:
      - https: {hostnames: [generativelanguage.googleapis.com]}
      - http: {hostnames: [nats.agents.svc.cluster.local], ports: [8080]}
      - tls: {hostnames: [github.com, "*.github.com"]}
```

The three protocols differ in what the gateway sees. `http` is cleartext, so the gateway decides on each request. `https` is intercepted: the gateway terminates TLS with a certificate from its own CA, decides on each request, and opens its own TLS connection to the destination; the sandbox trusts the gateway CA, so clients inside it do not notice. `tls` is passed through: the gateway decides once per connection on the server name of the ClientHello and forwards the bytes, so it works for any protocol over TLS and inspects nothing. For a given name and port one rule decides, the most specific pattern first, then a named port over all ports. Substrate rejects a policy with two rules on the same pattern and port.

A task without `spec.egress` gets no policy and no network, which is the safe default. Protocols where the server speaks first after connect, such as plain NATS, do not work through the gateway; use their WebSocket or HTTP forms.
