resource "aws_security_group" "main" {
  name   = "${var.name}-sg"
  vpc_id = var.vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_lambda_function" "main" {
  filename      = var.zip_path
  function_name = var.name
  role          = var.role_arn
  handler       = "index.handler"
  runtime       = "nodejs18.x"

  vpc_config {
    subnet_ids         = [var.subnet_id]
    security_group_ids = [aws_security_group.main.id]
  }
}
