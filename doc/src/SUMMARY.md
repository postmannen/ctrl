
# Introduction

- [Introduction](./introduction.md)
  
# User Guides

- [Install docker](./user_guide_install_docker.md)
- [Install host](./user_guide_install_host.md)
- [Install NATS Server](./install_nats_server.md)

# Core ctrl

- [Core overview](./core_overview.md)
- [Messaging](./core_messaging_overview.md)
  - [Message fields](./core_messaging_message_fields.md)
  - [Message jetstream/broadcast](./core_messaging_jetstream.md)
  - [Request Methods](./core_request_methods.md)
  - [{{CTRL_DATA}} variable](./core_messaging_CTRL_DATA.md)
  - [{{CTRL_FILE}} variable](./core_messaging_CTRL_FILE.md)
- [Nats timeouts](./core_nats_timeouts.md)
- [Startup folder](./core_startup_folder.md)
- [Errors](./core_errors.md)
- [central](./core_central.md)
  - [hello messages](./core_hello_messages.md)
  - [signing keys](./core_signing_keys.md)
  - [ACL](./core_acl.md)
  - [audit log](./core_audit_log.md)

# Examples standard messages

- [Http Get](./example_standard_reqhttpget.md)
- [Tail File](./example_standard_reqtailfile.md)
- [Cli Command](./example_standard_reqclicommand.md)
- [Cli Command Continously](./example_standard_reqclicommandcont.md)
- [Copy Src to Dst](./example_standard_reqcopysrc.md)
- [Send more messages at once](./example_standard_send_more_messages.md)

# Using ctrl

- [ctrl as github action runner](usecase-ctrl-as-github-action-runner.md)
- [ctrl as prometheus collector](usecase-ctrl-as-prometheus-collector.md)
- [ctrl as tcp forwarder for ssh](usecase-ctrl-as-tcp-forwarder-for-ssh.md)
