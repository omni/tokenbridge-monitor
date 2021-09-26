variable "gcp_credentials" {
  type    = string
  default = "./gcp_credentials.json"
}

variable "project" {
  type    = string
  default = "silicon-webbing-327211"
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "zone" {
  type    = string
  default = "europe-west1-b"
}

variable "ssh_user" {
  type    = string
  default = "root"
}

variable "ssh_key_file" {
  type    = string
  default = "./ssh_key.pub"
}
