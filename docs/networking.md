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
    hosts:
      - github.com
      - "*.github.com"
      - https://generativelanguage.googleapis.com
      - http://nats.agents.svc.cluster.local:8080
```

A host is a DNS name with an optional scheme and an optional port, and the scheme decides what the gateway sees. A name alone is TLS on port 443, passed through: the gateway decides once per connection on the server name of the ClientHello and forwards the bytes, so it works for any protocol over TLS and inspects nothing. `http://` is cleartext HTTP on port 80, so the gateway decides on each request. `https://` is HTTPS on port 443, intercepted: the gateway terminates TLS with a certificate from its own CA, decides on each request, and opens its own TLS connection to the destination; the sandbox trusts the gateway CA, so clients inside it do not notice. A port after the name replaces the default. Each host becomes one rule of the policy. For a given name and port one rule decides, the most specific pattern first. Substrate rejects a policy with two rules on the same pattern and port, so the API server refuses two hosts that name the same host and port.

A task without `spec.egress` gets no policy and no network, which is the safe default. Protocols where the server speaks first after connect, such as plain NATS, do not work through the gateway; use their WebSocket or HTTP forms.
