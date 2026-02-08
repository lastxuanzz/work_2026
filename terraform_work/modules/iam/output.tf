# output "user_arn" {
#   value = aws_iam_user.operator.arn
# }
output "role_arns" {
  value = {
    # role_1 = aws_iam_role.role_1.arn
    # role_2 = aws_iam_role.role_2.arn
    # role_3 = aws_iam_role.role_3.arn
    # role_4 = aws_iam_role.role_4.arn
    role_5 = aws_iam_role.role_5.arn
    role_6 = aws_iam_role.role_6.arn
  }
}
