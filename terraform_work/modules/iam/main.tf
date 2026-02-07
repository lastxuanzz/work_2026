# 1. User & Policy
resource "aws_iam_user" "operator" {
  name = var.iam_user_name
}

resource "aws_iam_policy" "user_assume_policy" {
  name = var.user_assume_policy_name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = "sts:AssumeRole"
      Resource = [
        aws_iam_role.role_1.arn,
        aws_iam_role.role_2.arn,
        aws_iam_role.role_3.arn
      ]
    }]
  })
}

resource "aws_iam_user_policy_attachment" "attach_user" {
  user       = aws_iam_user.operator.name
  policy_arn = aws_iam_policy.user_assume_policy.arn
}

# 2. Role 1 (Get S3 A)
resource "aws_iam_role" "role_1" {
  name = var.role_1_name
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { AWS = aws_iam_user.operator.arn }
    }]
  })
}

resource "aws_iam_role_policy" "role_1_p" {
  name = var.role_1_policy_name
  role = aws_iam_role.role_1.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow",
        Action = ["s3:GetObject", "s3:ListBucket"],
        Resource = [
          "arn:aws:s3:::${var.bucket_a_name}",
          "arn:aws:s3:::${var.bucket_a_name}/*"
        ]
      },
      {
        Effect   = "Allow",
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"],
        Resource = var.kms_key_arn
      }
    ]
  })
}

# 3. Role 2 (Put A to B)
resource "aws_iam_role" "role_2" {
  name = var.role_2_name
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { AWS = aws_iam_user.operator.arn }
    }]
  })
}

resource "aws_iam_role_policy" "role_2_p" {
  name = var.role_2_policy_name
  role = aws_iam_role.role_2.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = "s3:GetObject",
        Resource = "arn:aws:s3:::${var.bucket_a_name}/*"
      },
      {
        Effect   = "Allow",
        Action   = "s3:PutObject",
        Resource = "arn:aws:s3:::${var.bucket_b_name}/*"
      },
      {
        Effect   = "Allow",
        Action   = ["kms:Decrypt", "kms:Encrypt", "kms:GenerateDataKey"],
        Resource = var.kms_key_arn
      }
    ]
  })
}

# 4. Role 3 (Invoke Lambda)
resource "aws_iam_role" "role_3" {
  name = var.role_3_name
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { AWS = aws_iam_user.operator.arn }
    }]
  })
}

resource "aws_iam_role_policy" "role_3_p" {
  name = var.role_3_policy_name
  role = aws_iam_role.role_3.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow",
      Action   = "lambda:InvokeFunction",
      Resource = "arn:aws:lambda:*:*:function:${var.target_lambda_name}"
    }]
  })
}

# 5. Role 4 (Lambda Presign)
resource "aws_iam_role" "role_4" {
  name = var.role_4_name
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "role_4_p" {
  name = var.role_4_policy_name
  role = aws_iam_role.role_4.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = "s3:GetObject",
        Resource = "*"
      },
      {
        "Effect" : "Allow",
        "Action" : [ # 确认中
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ],
        "Resource" : "arn:aws:logs:*:*:*" # CloudWatch Logsへのアクセス許可
      }
    ]
  })
}

# 6. Role 5 (Replication C -> D)
resource "aws_iam_role" "role_5" {
  name = var.role_5_name
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { Service = "s3.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "role_5_p" {
  name = var.role_5_policy_name
  role = aws_iam_role.role_5.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = ["s3:GetReplicationConfiguration", "s3:ListBucket"],
        Resource = "arn:aws:s3:::${var.bucket_c_tokyo_name}"
      },
      {
        Effect   = "Allow",
        Action   = ["s3:GetObjectVersionForReplication", "s3:GetObjectVersionAcl"],
        Resource = "arn:aws:s3:::${var.bucket_c_tokyo_name}/*"
      },
      {
        Effect   = "Allow",
        Action   = ["s3:ReplicateObject", "s3:ReplicateDelete"],
        Resource = "arn:aws:s3:::${var.bucket_d_osaka_name}/*"
      }
    ]
  })
}

# 7. Role 6 (Replication D -> C)
resource "aws_iam_role" "role_6" {
  name = var.role_6_name
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { Service = "s3.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "role_6_p" {
  name = var.role_6_policy_name
  role = aws_iam_role.role_6.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow",
        Action   = ["s3:GetReplicationConfiguration", "s3:ListBucket"],
        Resource = "arn:aws:s3:::${var.bucket_d_osaka_name}"
      },
      {
        Effect   = "Allow",
        Action   = ["s3:GetObjectVersionForReplication", "s3:GetObjectVersionAcl"],
        Resource = "arn:aws:s3:::${var.bucket_d_osaka_name}/*"
      },
      {
        Effect   = "Allow",
        Action   = ["s3:ReplicateObject", "s3:ReplicateDelete"],
        Resource = "arn:aws:s3:::${var.bucket_c_tokyo_name}/*"
      }
    ]
  })
}
