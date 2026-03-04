resource "aws_cloudwatch_log_group" "main" {
  name              = "/aws/lambda/${var.func_name}"
  retention_in_days = 7
}
