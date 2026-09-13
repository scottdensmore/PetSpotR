# Backend configuration for remote OpenTofu state in Google Cloud Storage.
# To enable remote state:
#   tofu init -backend-config="bucket=<YOUR_STATE_BUCKET>" -backend-config="prefix=petspotr/state"
terraform {
  # backend "gcs" {}
}
