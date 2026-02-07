module "iam" {
  source              = "../../modules/iam"
  bucket_a_name       = local.bucket_a
  bucket_b_name       = local.bucket_b
  bucket_c_tokyo_name = local.bucket_c
  bucket_d_osaka_name = local.bucket_d
  target_lambda_name  = local.lambda

  # User and Policy names
  iam_user_name            = local.iam_user_name
  user_assume_policy_name  = local.user_assume_policy
  role_1_name              = local.role_1_name
  role_1_policy_name       = local.role_1_policy
  role_2_name              = local.role_2_name
  role_2_policy_name       = local.role_2_policy
  role_3_name              = local.role_3_name
  role_3_policy_name       = local.role_3_policy
  kms_key_arn              = local.kms_key_arn
  role_4_name              = local.role_4_name
  role_4_policy_name       = local.role_4_policy
  role_5_name              = local.role_5_name
  role_5_policy_name       = local.role_5_policy
  role_6_name              = local.role_6_name
  role_6_policy_name       = local.role_6_policy
}
