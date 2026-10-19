# Wait until web-1 has joined; when it is destroyed, remove it from the fleet.
resource "gardience_machine" "web1" {
  name         = "web-1"
  join_timeout = "20m"
  depends_on   = [aws_instance.web1]
}
