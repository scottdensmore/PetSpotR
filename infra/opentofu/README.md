# Google provider 8 migration validation

The root module selects Google provider `~> 8.9`; the lockfile selects `8.9.0`.
Provider installation and validation are separate from deploying these resources.
Do not initialize a remote backend or run a cloud plan/apply as part of dependency
validation.

## Compatibility review from provider 5

The provider 6, 7 and 8 upgrade guides were reviewed against the resources and
arguments in this module:

- Provider 6 enables the `goog-terraform-provisioned` label by default. The root
  provider explicitly disables this attribution label to preserve the provider
  5 labeling behavior.
- Provider 6 adds deletion protection to Cloud Run services, enabled by default.
  This migration retains that protection. It does not opt out or change deployed
  security settings. Any later resource replacement/deletion needs a separate
  operational review; schema validation does not establish that a real-state
  plan has no changes.
- Cloud Run API v2 rejects reserved `run.googleapis.com` annotations. The
  inference service now expresses its existing BETA launch-stage intent through
  the dedicated service-level `launch_stage` field, replacing the invalid
  template annotation.
- Cloud Run container environment blocks became sets. This configuration does not
  index individual environment entries. The inference service keeps explicit
  startup/liveness probes; no additional probe removal or change is introduced.
- Provider 7 removes the default for project-service `disable_on_destroy`. Every
  project-service resource here already sets it to `false` explicitly.
- The removed resources/arguments in the provider 8 guide are not declared here.
  The storage retention-period type change in provider 7 is also unused.

These checks do not prove compatibility with unseen cloud state or remote API
behavior. No backend, credentials, cloud data or state is needed for the following
configuration/schema validation.

## Local validation

Use an existing OpenTofu binary and an isolated task data directory. On War-Machine
an existing binary is OpenTofu 1.12.6, darwin_amd64. Do not invoke tool-manager
fallbacks or install another host toolchain.

After approval to fetch the lockfile-pinned provider executable:

```sh
# Run from this directory. Set these to your existing binary and private task folder.
"$TOFU_BIN" fmt -check -recursive .
TF_DATA_DIR="$VALIDATION_DATA_DIR" "$TOFU_BIN" init -backend=false -input=false -lockfile=readonly
TF_DATA_DIR="$VALIDATION_DATA_DIR" "$TOFU_BIN" validate -no-color
TF_DATA_DIR="$VALIDATION_DATA_DIR" "$TOFU_BIN" providers schema -json > "$VALIDATION_SCHEMA_JSON"
```

The provider source is `registry.opentofu.org/hashicorp/google`. OpenTofu verifies
provider package checksums/signatures against the registry and committed lockfile.
For the reviewed macOS Intel package, registry metadata identifies:

- Release: `https://github.com/opentofu/terraform-provider-google/releases/tag/v8.9.0`
- File: `terraform-provider-google_8.9.0_darwin_amd64.zip`
- SHA256: `cc4f83e4cedc15e7ba69921aca72df0c66f543fd1688923828787cb08792a6ae`

If only the lockfile's recorded constraint needs regeneration after the explicit
configuration constraint change, run the same isolated `init` without
`-lockfile=readonly` and without `-upgrade`. Review the result: provider version
must stay `8.9.0`, checksum values must not be hand-edited, and only metadata
consistent with the configuration may change.

No production/state plan, infrastructure apply, deployment or cloud integration
success is implied by a successful `validate`/schema check.

## Upstream references

- [Provider 6 upgrade guide](https://github.com/hashicorp/terraform-provider-google/blob/v8.9.0/website/docs/guides/version_6_upgrade.html.markdown)
- [Provider 7 upgrade guide](https://github.com/hashicorp/terraform-provider-google/blob/v8.9.0/website/docs/guides/version_7_upgrade.html.markdown)
- [Provider 8 upgrade guide](https://github.com/hashicorp/terraform-provider-google/blob/v8.9.0/website/docs/guides/version_8_upgrade.html.markdown)

- [Cloud Run v2 service reference](https://github.com/hashicorp/terraform-provider-google/blob/v8.9.0/website/docs/r/cloud_run_v2_service.html.markdown)
