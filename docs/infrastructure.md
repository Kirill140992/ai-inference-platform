# Infrastructure as Code (IaC)

The platform's underlying infrastructure (currently running on Hetzner Cloud) is provisioned, managed, and version-controlled using **Terraform**.

## State Management & Security (HCP Terraform)

Local Terraform state files (`.tfstate`) often contain sensitive information in plaintext, including IP addresses, metadata, and infrastructure tokens. To mitigate the risk of credential leakage and state corruption, the platform utilizes **HashiCorp Cloud Platform (HCP) Terraform** for remote state management.

* **Remote Execution & Encryption:** The Terraform state is securely stored remotely and encrypted at rest.
* **Access Control & Locking:** HCP Terraform provides state locking during operations (`terraform apply`), preventing race conditions and concurrent modifications if multiple engineers work on the infrastructure.
* **Separation of Concerns:** Developers do not need high-privileged Cloud API tokens on their local machines; the remote workspace acts as the secure execution environment.

![HCP Terraform Workspace](../architecture/terraform-cloud.png)