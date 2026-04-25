# Future Mesh Discovery Design

This is a future design note. The current implementation supports static seed
nodes through `--seed-nodes`. The ideas below describe how EventMesh could grow
from static seed discovery into a more robust peer-discovery system.

## Goals

- Find enough peers quickly to form a useful mesh.
- Avoid central coordination on the data path.
- Keep operations simple for local, VM, and Kubernetes deployments.
- Preserve cluster isolation and peer identity once mTLS is implemented.

## Discovery Tiers

### Tier 1: Static Seeds

Current implementation:

```bash
./bin/eventmesh --http --node-id node2 --peer-listen :9091 \
  --seed-nodes localhost:9090
```

This is the right first mechanism because it is explicit, easy to test, and easy
to reason about.

### Tier 2: DNS Seeds

Future option:

- `seeds.mesh.example.com` returns A/AAAA records for seed nodes.
- Optional SRV records can advertise peer ports.
- Operators rotate seed nodes by updating DNS.

### Tier 3: Local And Platform Discovery

Future options:

- mDNS for local development on one subnet
- Kubernetes headless Services
- cloud instance tags
- a small HTTPS rendezvous endpoint returning candidate peers

## Join Flow

Future target flow:

1. Load node identity and cluster policy.
2. Collect bootstrap candidates from static seeds, DNS, and optional providers.
3. Deduplicate candidates.
4. Dial candidates in parallel with jitter and backoff.
5. Validate peer identity and cluster membership.
6. Exchange a bounded peer list.
7. Maintain an active peer view and a passive candidate view.

## Suggested Defaults

```yaml
mesh:
  degree:
    target: 6
    low: 4
    high: 10
  passive_view: 30
  pex_on_join: 16
  shuffle_interval: "30s"
  heartbeat: "2s"
  backoff:
    min: "500ms"
    max: "60s"
```

## Security Direction

When peer security is added:

- mTLS should be the default peer transport.
- Certificates should encode node identity and cluster identity.
- The handshake should reject mismatched cluster IDs.
- Peer exchange should only accept peers that prove membership.

Smallstep, SPIFFE/SPIRE, or another PKI mechanism can be evaluated when this
work becomes active.

## Not Planned For Early Versions

- DHT-based discovery
- service-mesh dependency
- TURN/WebRTC-style traversal
- complex NAT traversal before static seeds and outbound connections are proven
