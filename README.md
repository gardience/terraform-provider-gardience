# Terraform provider for Gardience

[Gardience](https://gardience.com) is a supported console for Windows and Linux machines on open-source
[Salt](https://saltproject.io): patching, a scored STIG/CIS-aligned security baseline, drift repair,
event-driven automation, two-person approval and a tamper-evident audit log.

This provider lets Terraform (and OpenTofu) hand every machine it builds to Gardience on first boot:

- **`gardience_enrollment`**: first-boot scripts for `user_data` (Linux sh, Windows PowerShell). New machines
  install the pinned, hash-checked Salt agent and join without anyone approving them.
- **`gardience_event_rule`**: event-driven automation. `join` applies a saved state to each machine the moment
  it first joins; `service` restarts a stopped service in seconds; `disk` warns when a disk fills up.
- **`gardience_state`**: saved Salt states, checked by Gardience before they are published.
- **`gardience_node_group`**: custom node groups.
- **`gardience_machine`**: waits until a machine has joined; removes it from the fleet on `terraform destroy`.
- Data sources **`gardience_bootstrap_script`** (a script per machine) and **`gardience_machine`**.

Every change is recorded in Gardience's audit log under the API key's owner.

## Use it

```hcl
terraform {
  required_providers {
    gardience = { source = "gardience/gardience" }
  }
}

provider "gardience" {} # reads GARDIENCE_URL and GARDIENCE_API_KEY
```

The key is a Gardience **configure** key (Gardience 1.5.0 or later): an admin creates it in
**Settings > API keys**. It manages configuration and removes machines, and never runs anything on the fleet.

Full documentation: the [registry docs](docs/index.md) in this repository, and
[gardience.com/docs](https://gardience.com/docs/) (Clients > Terraform). A complete AWS example is in
[examples/complete-aws](examples/complete-aws/main.tf).

## Build and test

```bash
go build ./...
go vet ./...
```

The end-to-end test (a real Gardience install, a machine built by Terraform that joins and is configured by a join
rule, then destroyed) lives with the Gardience server source and runs in every Gardience release check.

## Verify a release

Each release has a `SHA256SUMS` file signed with the Gardience provider signing key, fingerprint
`7D4F EF04 FF05 7ECC 85E6 1D37 3F4E 9402 C40E 7D3E`. The Terraform Registry and OpenTofu check the signature when they
install the provider. The public key is in [signing-key.asc](signing-key.asc).

## License

MIT. Salt is a trademark of its owner; Gardience is independent of the Salt Project and Broadcom.
