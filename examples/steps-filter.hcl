job {
  name = "steps-filter-demo"
}

# Intermediate step — used to feed data to the next step.
step "static" "regions" {
  value    = <<-JSON
    {"items": ["us-east-1", "eu-west-1", "ap-southeast-1"]}
  JSON
  parse_as = "json"
}

# The step we actually care about.
step "exec" "echo_date" {
  program = ["date", "+%Y-%m-%d"]
  format  = "raw"
}

# Another useful result.
step "static" "metadata" {
  value    = <<-JSON
    {"version": "1.0", "source": "steps-filter-demo"}
  JSON
  parse_as = "json"
}

output {
  # Only include echo_date and metadata — regions is intermediate.
  steps = [step.exec.echo_date, step.static.metadata]

  sink "filesystem" {
    path = "./output"
  }
}
