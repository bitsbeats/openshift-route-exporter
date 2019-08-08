# Openshift route exporter

A small tool to export openshift routed to a monitoring tool.

## Depricated

See https://github.com/bitsbeats/openshift-route-monitor

## Usage

Configuration is handled via a single yaml file and kubeconfigs:

```yaml
targets:
  - kubeconfig: /etc/openshift-route-exporter/devcluster.kubeconfig
    labels:
      cluster: devcluster
  - kubeconfig: /etc/openshift-route-exporter/prodcluster.kubeconfig
    labels:
      cluster: prodcluster
exporter: prometheus
export_dir: /etc/prometheus/blackbox.d
```

Default location for the config is `/etc/openshift-route-exporter/config.yml`.
One may change this using the `CONFIG` environment variable.

Prometheus configuration:

```
- job_name: blackbox_http_2xx
  metrics_path: "/probe"
  file_sd_configs:
  - files:
    - "/etc/prometheus/blackbox.json.d/*.json"
  relabel_configs:
  - source_labels:
    - __address__
    target_label: __param_target
  - source_labels:
    - __param_target
    target_label: instance
  - target_label: __address__
    replacement: 127.0.0.1:9115

```

You will need a blackbox exporter up and running.
