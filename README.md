# Openshift route exporter

A small tool to export openshift routed to a monitoring tool.

## Usage

Configuration is handled via environment variables.

* `KUBE_CONFIG`: a single kube config, or multiple `;` seperated. Default `$HOME/.kube/config`
* `EXPORT_DIR`: directory where configuration files will be created. Defautl `/etc/prometheus/kube.d/`
* `EXPORTER`: exporter to use, currently only `prometheus` is available. Default `prometheus`




