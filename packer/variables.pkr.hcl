variable "accelerator" {
  type    = string
  default = "kvm"
}

variable "arch" {
  type    = string
  default = "amd64"
}

variable "aws_access_key" {
  type = string
  default = env("AWS_ACCESS_KEY_ID")
  sensitive = true
}

variable "aws_secret_key" {
  type    = string
  default = env("AWS_SECRET_ACCESS_KEY")
  sensitive = true
}

variable "build" {
  type    = string
  default = "dev"
}

variable "cos_version" {
  type    = string
  default = "0.5.5"
}

variable "aws_cos_install_args" {
  type = string
  default = "cos-deploy --docker-image quay.io/costoolkit/releases-opensuse:cos-system-0.5.5"
}

variable "cpus" {
  type    = string
  default = "3"
}

variable "disk_size" {
  type    = string
  default = "50000"
}

variable "feature" {
  type    = string
  default = ""
}

variable "flavor" {
  type    = string
  default = "leap"
}

variable "iso" {
  type    = string
  default = ""
}

variable "memory" {
  type    = string
  default = "8192"
}

variable "aws_region" {
  type    = string
  default = env("AWS_DEFAULT_REGION")
  sensitive = true
}

variable "root_password" {
  type    = string
  default = "cos"
}

variable "root_username" {
  type    = string
  default = "root"
}

variable "sleep" {
  type    = string
  default = "30s"
}