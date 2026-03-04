resource "aws_route_table" "main" {
  vpc_id = var.vpc_id
  tags = {
    Name = var.name
  }
}

resource "aws_route" "default" {
  route_table_id         = aws_route_table.main.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = var.gateway_id
  nat_gateway_id         = var.nat_gateway_id
}

resource "aws_route_table_association" "main" {
  subnet_id      = var.subnet_id
  route_table_id = aws_route_table.main.id
}
