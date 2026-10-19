resource "gardience_node_group" "web" {
  name    = "web-servers"
  kind    = "match"
  pattern = "web-*"
}

resource "gardience_node_group" "rhel" {
  name = "rhel-family"
  kind = "os"
  os   = "RHEL family"
}
