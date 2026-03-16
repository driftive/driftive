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
