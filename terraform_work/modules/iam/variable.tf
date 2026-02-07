variable "bucket_a_name" { type = string }
variable "bucket_b_name" { type = string }
variable "bucket_c_tokyo_name" { type = string }
variable "bucket_d_osaka_name" { type = string }
variable "target_lambda_name" { type = string }

# User and Policy names
variable "iam_user_name" { type = string }
variable "user_assume_policy_name" { type = string }
variable "role_1_name" { type = string }
variable "role_1_policy_name" { type = string }
variable "role_2_name" { type = string }
variable "role_2_policy_name" { type = string }
variable "role_3_name" { type = string }
variable "role_3_policy_name" { type = string }
variable "role_4_name" { type = string }
variable "role_4_policy_name" { type = string }
variable "role_5_name" { type = string }
variable "role_5_policy_name" { type = string }
variable "role_6_name" { type = string }
variable "role_6_policy_name" { type = string }

variable "kms_key_arn" {
  description = "KMS key ARN for encrypting and decrypting S3 objects"
  type        = string
}
