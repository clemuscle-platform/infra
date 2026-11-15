# Buckets Hetzner Object Storage (ADR-0008). Les clés d'accès S3 se créent dans la console Hetzner.
# Le bucket de l'état OpenTofu (<prefix>-tfstate) n'est PAS géré ici : il doit exister avant tofu init.
#
# prevent_destroy : « les données ne sont jamais jetables ». Supprimer un bucket exige de retirer
# cette protection dans une PR relue par un humain (contrôle tofu-destroy-guard).

provider "aws" {
  region     = var.location
  access_key = var.s3_access_key
  secret_key = var.s3_secret_key

  endpoints {
    s3 = "https://${local.s3_endpoint_host}"
  }

  skip_credentials_validation = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true
  skip_region_validation      = true
  s3_use_path_style           = true
}

locals {
  buckets = {
    pg   = { versioning = true, noncurrent_days = 30 } # WAL et sauvegardes CNPG
    etcd = { versioning = false, noncurrent_days = 0 } # instantanés k3s
    o2   = { versioning = false, noncurrent_days = 0 } # données OpenObserve
    apps = { versioning = true, noncurrent_days = 30 } # fichiers des applications
  }
}

resource "aws_s3_bucket" "platform" {
  for_each = local.buckets
  bucket   = "${var.bucket_prefix}-${each.key}"

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_s3_bucket_versioning" "platform" {
  for_each = { for k, v in local.buckets : k => v if v.versioning }
  bucket   = aws_s3_bucket.platform[each.key].id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "platform" {
  for_each = { for k, v in local.buckets : k => v if v.noncurrent_days > 0 }
  bucket   = aws_s3_bucket.platform[each.key].id

  rule {
    id     = "expire-noncurrent"
    status = "Enabled"
    filter {}
    noncurrent_version_expiration {
      noncurrent_days = each.value.noncurrent_days
    }
  }

  depends_on = [aws_s3_bucket_versioning.platform]
}
