# Saved states are checked by Gardience before they are published. Saving never applies one:
# people apply it from the console, or a join rule applies it to each new machine.
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
