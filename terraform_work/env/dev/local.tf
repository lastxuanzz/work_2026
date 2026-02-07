locals {
  bucket_a = "my-bucket-a-dev"
  bucket_b = "my-bucket-b-dev"
  bucket_c = "my-bucket-tokyo-c-dev"
  bucket_d = "my-bucket-osaka-d-dev"
  lambda   = "my-function-dev"

  # User and Policy names
  iam_user_name          = "app-operator-user-dev"
  user_assume_policy     = "user-assume-roles-policy-dev"
  role_1_name            = "role-1-read-bucket-a-dev"
  role_1_policy          = "read-a-dev"
  role_2_name            = "role-2-copy-a-to-b-dev"
  role_2_policy          = "copy-a-to-b-dev"
  role_3_name            = "role-3-invoke-lambda-dev"
  role_3_policy          = "invoke-lambda-dev"
  kms_key_arn            = "arn:aws:kms:us-xxx-x:123456789012:key/abcd1234-a123-456a-a12b-a123b4cd56ef"
  role_4_name            = "role-4-lambda-presign-dev"
  role_4_policy          = "s3-access-dev"
  role_5_name            = "role-5-rep-c-to-d-dev"
  role_5_policy          = "rep-c-to-d-dev"
  role_6_name            = "role-6-rep-d-to-c-dev"
  role_6_policy          = "rep-d-to-c-dev"
}
