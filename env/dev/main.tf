module "vpc" {
  source = "../../modules/vpc"
  cidr   = "10.0.0.0/16"
  name   = "${local.project_name}-vpc"
}

module "igw" {
  source = "../../modules/igw"
  vpc_id = module.vpc.id
  name   = "${local.project_name}-igw"
}

module "pub_sub" {
  source    = "../../modules/subnets"
  vpc_id    = module.vpc.id
  cidr      = "10.0.1.0/24"
  is_public = true
  az        = local.az
  name      = "public-subnet"
}

module "priv_sub" {
  source    = "../../modules/subnets"
  vpc_id    = module.vpc.id
  cidr      = "10.0.2.0/24"
  is_public = false
  az        = local.az
  name      = "private-subnet"
}

module "eip" {
  source = "../../modules/eip"
  name   = "nat-eip"
}

module "nat" {
  source           = "../../modules/nat"
  eip_id           = module.eip.id
  public_subnet_id = module.pub_sub.id
  name             = "main-nat"
}

module "rt_pub" {
  source     = "../../modules/route_tables"
  vpc_id     = module.vpc.id
  subnet_id  = module.pub_sub.id
  gateway_id = module.igw.id
  name       = "public-rt"
  nat_gateway_id = null
}

module "rt_priv" {
  source         = "../../modules/route_tables"
  vpc_id         = module.vpc.id
  subnet_id      = module.priv_sub.id
  nat_gateway_id = module.nat.id
  name           = "private-rt"
  gateway_id     = null
}

module "role" {
  source  = "../../modules/iam_role"
  name    = "lambda-vpc-role"
  service = "lambda.amazonaws.com"
}

module "policy" {
  source    = "../../modules/iam_policy"
  role_name = module.role.name
  name      = "lambda-extended-policy"
  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action   = ["ssm:GetParameter", "sns:Publish", "logs:*", "ec2:CreateNetworkInterface", "ec2:DescribeNetworkInterfaces", "ec2:DeleteNetworkInterface"]
        Effect   = "Allow"
        Resource = "*"
      }
    ]
  })
}

module "ssm" {
  source = "../../modules/ssm_parameter"
  name   = "/app/api_key"
  value  = "dummy-key-123"
}

module "sns" {
  source = "../../modules/sns"
  name   = "app-notifications"
  email  = "xxx@xxx.com"
}

module "logs" {
  source    = "../../modules/cloudwatch_logs"
  func_name = "static-ip-worker"
}

module "lambda" {
  source    = "../../modules/lambda"
  name      = "static-ip-worker"
  vpc_id    = module.vpc.id
  subnet_id = module.priv_sub.id
  role_arn  = module.role.arn
  zip_path  = "../../modules/lambda/code/lambda.zip"
}

module "scheduler" {
  source      = "../../modules/eventbridge"
  name        = "lambda-trigger"
  schedule    = "rate(10 minutes)"
  lambda_arn  = module.lambda.arn
  lambda_name = module.lambda.name
}
