resource "aws_ssm_parameter" "main" {
  name  = var.name
  type  = "SecureString"
  value = var.value
}
