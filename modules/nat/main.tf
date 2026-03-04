resource "aws_nat_gateway" "main" {
  allocation_id = var.eip_id
  subnet_id     = var.public_subnet_id
  tags = {
    Name = var.name
  }
}
