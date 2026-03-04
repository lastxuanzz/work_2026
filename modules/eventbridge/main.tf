resource "aws_cloudwatch_event_rule" "sched" {
  name                = var.name
  schedule_expression = var.schedule
}

resource "aws_cloudwatch_event_target" "lambda" {
  rule      = aws_cloudwatch_event_rule.sched.name
  target_id = "SendToLambda"
  arn       = var.lambda_arn
}

resource "aws_lambda_permission" "allow_eb" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = var.lambda_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.sched.arn
}
