variable "REGISTRY" { default = "ghcr.io/sudowritecode" }
variable "VERSION" { default = "dev" }

group "default" { targets = ["blackark", "blackark-control", "blackark-agent"] }

target "binary" {
  context = "."
  dockerfile = "Dockerfile"
  platforms = ["linux/amd64", "linux/arm64"]
}

target "blackark" {
  inherits = ["binary"]
  args = { TARGET = "blackark", VERSION = "${VERSION}" }
  tags = ["${REGISTRY}/blackark:${VERSION}"]
}
target "blackark-control" {
  inherits = ["binary"]
  args = { TARGET = "blackark-control", VERSION = "${VERSION}" }
  tags = ["${REGISTRY}/blackark-control:${VERSION}"]
}
target "blackark-agent" {
  inherits = ["binary"]
  args = { TARGET = "blackark-agent", VERSION = "${VERSION}" }
  tags = ["${REGISTRY}/blackark-agent:${VERSION}"]
}

