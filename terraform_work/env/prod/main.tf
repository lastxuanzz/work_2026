module "iam" {
  source              = "../../modules/iam"
  bucket_a_name       = local.bucket_a
  bucket_b_name       = local.bucket_b
  bucket_c_tokyo_name = local.bucket_c
  bucket_d_osaka_name = local.bucket_d
  target_lambda_name  = local.lambda
}
