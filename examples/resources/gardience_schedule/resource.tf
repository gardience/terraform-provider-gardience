# Runs the runbook every Monday, Wednesday, and Friday at 02:30 Central.
# It starts turned off: an admin turns it on in the console (Automation > Schedules).
resource "gardience_schedule" "nightly" {
  id     = "nightly-restart"
  name   = "Nightly nginx restart"
  ref    = gardience_runbook.restart.id
  inputs = { service = "nginx", group = "Linux" }
  days   = ["mon", "wed", "fri"]
  time   = "02:30"
  tz     = "America/Chicago"
}

output "nightly_is_on" {
  value = gardience_schedule.nightly.enabled
}
