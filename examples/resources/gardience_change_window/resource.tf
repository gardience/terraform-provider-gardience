# Changes to Linux machines wait for the weekend evening window.
resource "gardience_change_window" "linux_weekend" {
  id        = "linux-weekend"
  nodegroup = "Linux"
  days      = ["sat", "sun"]
  start     = "22:00"
  end       = "23:59"
  tz        = "America/Chicago"
}
