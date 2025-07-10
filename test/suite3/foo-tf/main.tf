resource "null_resource" "foo" {
  triggers = {
    foo = "bar2"
  }
}

resource "null_resource" "foo2" {
  triggers = {
    foo = "bar2"
  }
}

data "google_container_cluster" "foo" {
  name     = "foo"
  project = "foooooo-123456"
  location = "us-central1"
}
