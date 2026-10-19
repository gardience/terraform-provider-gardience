# Machines built with this enrollment join Gardience on their own and land in the "web" group.
resource "gardience_enrollment" "web" {
  label  = "web tier (terraform)"
  hours  = 2160 # 90 days; Terraform opens a new one when it expires
  groups = ["web"]
}

# Any machine can use the generic script as its user_data:
#   user_data = gardience_enrollment.web.linux_script
