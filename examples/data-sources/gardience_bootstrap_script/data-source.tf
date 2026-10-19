data "gardience_bootstrap_script" "web1" {
  enrollment_id = gardience_enrollment.web.id
  os            = "linux"
  node          = "web-1"
  groups        = ["blue"]
}

resource "aws_instance" "web1" {
  ami           = var.ami
  instance_type = "t3.small"
  user_data     = data.gardience_bootstrap_script.web1.content
}

# Windows on AWS: user_data = "<powershell>${data.gardience_bootstrap_script.win1.content}</powershell>"
