# A form people fill in to restart one service on a node group.
resource "gardience_runbook" "restart" {
  id          = "restart-service"
  name        = "Restart a service"
  description = "Restarts one service on every machine in a node group"
  fun         = "service.restart"
  args        = ["{{service}}"]
  target      = "{{group}}"
  target_type = "nodegroup"
  survey = [
    { name = "service", label = "Service", default = "nginx" },
    { name = "group", label = "Node group", type = "nodegroup", default = "Linux" },
  ]
}
