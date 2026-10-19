#!/bin/sh
# Make a signed provider release for the Terraform Registry and OpenTofu:  tools/release.sh v0.1.0
# The signing key's passphrase comes from the macOS Keychain, only for this run.
set -eu
TAG="$1"
git diff --quiet && git diff --cached --quiet || { echo "refusing: uncommitted changes"; exit 1; }
go vet ./... && go build ./...
export GPG_FINGERPRINT=7D4FEF04FF057ECC85E61D373F4E9402C40E7D3E
GPG_PASSPHRASE="$(security find-generic-password -a gardience -s gardience-terraform-provider-gpg -w)"; export GPG_PASSPHRASE
GITHUB_TOKEN="$(gh auth token)"; export GITHUB_TOKEN
git tag -a "$TAG" -m "terraform-provider-gardience $TAG" 2>/dev/null || echo "tag $TAG exists"
git push origin "$TAG"
goreleaser release --clean
