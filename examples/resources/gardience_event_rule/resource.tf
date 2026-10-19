# Restart nginx within seconds whenever it stops on a Linux machine.
resource "gardience_event_rule" "keep_nginx" {
  id      = "keep-nginx"
  kind    = "service"
  os      = "linux"
  service = "nginx"
}

# Warn when C: is more than 90 percent full on a Windows machine.
resource "gardience_event_rule" "c_drive" {
  id      = "c-drive"
  kind    = "disk"
  os      = "windows"
  path    = "C:"
  percent = 90
}

# Configure every new machine in "web" the moment it first joins.
resource "gardience_event_rule" "web_join" {
  id    = "web-join"
  kind  = "join"
  scope = "group"
  group = "web"
  state = gardience_state.web_base.name
}
