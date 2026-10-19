# Three web servers on AWS that join Gardience, get their baseline on first boot, and leave the fleet on destroy.
terraform {
  required_providers {
    gardience = { source = "gardience/gardience" }
    aws       = { source = "hashicorp/aws" }
  }
}

provider "gardience" {} # GARDIENCE_URL and GARDIENCE_API_KEY
provider "aws" { region = "us-east-1" }

variable "ami" { description = "An Ubuntu 24.04 or RHEL 9 image" }

resource "gardience_enrollment" "web" {
  label  = "web tier"
  hours  = 2160
  groups = ["web"]
}

resource "gardience_state" "web_base" {
  name    = "web_base"
  content = <<-YAML
    nginx:
      pkg.installed: []
      service.running:
        - enable: True
        - require:
          - pkg: nginx
  YAML
}

resource "gardience_event_rule" "web_join" {
  id         = "web-join"
  kind       = "join"
  scope      = "group"
  group      = "web"
  state      = gardience_state.web_base.name
  depends_on = [gardience_enrollment.web]
}

resource "gardience_event_rule" "keep_nginx" {
  id      = "keep-nginx"
  kind    = "service"
  os      = "linux"
  service = "nginx"
}

data "gardience_bootstrap_script" "web" {
  count         = 3
  enrollment_id = gardience_enrollment.web.id
  os            = "linux"
  node          = "web-${count.index + 1}"
}

resource "aws_instance" "web" {
  count         = 3
  ami           = var.ami
  instance_type = "t3.small"
  user_data     = data.gardience_bootstrap_script.web[count.index].content
  tags          = { Name = "web-${count.index + 1}" }
}

resource "gardience_machine" "web" {
  count      = 3
  name       = "web-${count.index + 1}"
  depends_on = [aws_instance.web, gardience_event_rule.web_join]
}
