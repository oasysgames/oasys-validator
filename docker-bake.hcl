variable "REGISTRY" {
  default = "localhost"
}

variable "REPOSITORY" {
  default = "oasysgames/oasys-validator"
}

variable "GIT_COMMIT" {
  default = "dev"
}

// The default version to embed in the built images.
// During CI release builds this is set to <<pipeline.git.tag>>
variable "GIT_VERSION" {
  default = "v0.0.0"
}

variable "IMAGE_TAGS" {
  default = "${GIT_COMMIT}" // split by ","
}

variable "PLATFORMS" {
  // You can override this as "linux/amd64,linux/arm64".
  // Only a specify a single platform when `--load` ing into docker.
  // Multi-platform is supported when outputting to disk or pushing to a registry.
  // Multi-platform builds can be tested locally with:  --set="*.output=type=image,push=false"
  default = ""
}

group "default" {
  targets = ["geth", "binaries"]
}

target "geth" {
  dockerfile = "Dockerfile"
  context = "."
  args = {
    COMMIT = "${GIT_COMMIT}"
    VERSION = "${GIT_VERSION}"
  }
  platforms = split(",", PLATFORMS)
  tags = [for tag in split(",", IMAGE_TAGS) : "${REGISTRY}/${REPOSITORY}:${tag}"]
}

target "binaries" {
  dockerfile = "Dockerfile"
  target = "binaries"
  context = "."
  args = {
    VERSION = "${GIT_VERSION}"
  }
  platforms = split(",", PLATFORMS)
}
