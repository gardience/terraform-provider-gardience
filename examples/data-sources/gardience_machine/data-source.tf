data "gardience_machine" "web1" {
  name = "web-1"
}

output "web1_compliant" {
  value = data.gardience_machine.web1.compliant
}
