resource "aws_iam_policy" "main" {
  name   = var.name
  policy = var.policy_json
}

resource "aws_iam_role_policy_attachment" "main" {
  role       = var.role_name
  policy_arn = aws_iam_policy.main.arn
}
