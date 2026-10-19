terraform {
  required_providers {
    gardience = {
      source = "gardience/gardience"
    }
  }
}

# The address and a configure key (an admin makes one in Settings > API keys).
# Prefer the GARDIENCE_URL and GARDIENCE_API_KEY environment variables over writing them here.
provider "gardience" {
  url = "https://gardience.example.com"
}
