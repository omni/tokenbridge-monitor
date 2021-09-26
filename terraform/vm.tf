resource "google_compute_instance" "amb_monitor" {
  name         = "amb-monitor"
  machine_type = "e2-standard-2"

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2004-lts"
      size  = 100
      type  = "pd-ssd"
    }
  }

  network_interface {
    network = google_compute_network.vpc_network.self_link
    access_config {
    }
  }

  tags = ["https", "ssh"]

  metadata = {
    ssh-keys = "${var.ssh_user}:${file(var.ssh_key_file)}"
  }
}

output "amb_monitor_ip" {
  value = google_compute_instance.amb_monitor.network_interface.0.access_config.0.nat_ip
}
