resource "aws_subnet" "main" {
  vpc_id                  = var.vpc_id
  cidr_block              = var.cidr
  map_public_ip_on_launch = var.is_public
  availability_zone       = var.az
  tags = {
    Name = var.name
  }
}
