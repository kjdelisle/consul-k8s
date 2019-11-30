package connectinject

import (
	"bytes"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
)

type containerSidecarCommandData struct {
	AuthMethod         string
	EnvoyCertVolume    string // Secret volume to mount to the pod (must exist within the pod namespace).
	EnvoyCAFile        string // CA cert filename within the secret volume.
	EnvoyClientCert    string // TLS client cert filename within the secret volume.
	EnvoyClientKey     string // TLS client key filename within the secret volume.
	EnvoyTLSServerName string // TLS Server Name (SNI) for connecting to Consul.
}

func (h *Handler) containerSidecar(pod *corev1.Pod) (corev1.Container, error) {

	// Render the command
	var buf bytes.Buffer
	tpl := template.Must(template.New("root").Parse(strings.TrimSpace(
		sidecarPreStopCommandTpl)))
	err := tpl.Execute(&buf, containerSidecarCommandData{
		AuthMethod:      h.AuthMethod,
		EnvoyCertVolume: h.EnvoyCertVolume,
	})
	if err != nil {
		return corev1.Container{}, err
	}

	volMounts := []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/consul/connect-inject",
		},
	}

	if h.EnvoyCertVolume != "" {
		volMounts = append(volMounts, corev1.VolumeMount{
			Name:      h.EnvoyCertVolume,
			MountPath: "/consul/connect-inject/tls",
		})
	}

	return corev1.Container{
		Name:  "consul-connect-envoy-sidecar",
		Image: h.ImageEnvoy,
		Env: []corev1.EnvVar{
			{
				Name: "HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
				},
			},
		},
		VolumeMounts: volMounts,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-ec",
						buf.String(),
					},
				},
			},
		},
		Command: []string{
			"envoy",
			"--max-obj-name-len", "256",
			"--config-path", "/consul/connect-inject/envoy-bootstrap.yaml",
		},
	}, nil
}

const sidecarPreStopCommandTpl = `
{{- if .EnvoyCertVolume }}
export CONSUL_HTTP_ADDR="https://${HOST_IP}:8501"
{{- if .EnvoyTLSServerName }}
export CONSUL_TLS_SERVER_NAME="{{ .EnvoyTLSServerName }}"
{{- end }}
{{- else }}
export CONSUL_HTTP_ADDR="${HOST_IP}:8500"
{{- end }}
/consul/connect-inject/consul services deregister \
  {{- if .AuthMethod }}
  -token-file="/consul/connect-inject/acl-token" \
  {{- end }}
  {{- if .EnvoyCertVolume }}
  {{- if .EnvoyCAFile }}
  -ca-file="/consul/connect-inject/tls/ca.crt" \
  {{- end }}
  {{- if .EnvoyClientCert }}
  -client-cert="/consul/connect-inject/tls/tls.crt" \
  {{- end }} 
  {{- if .EnvoyClientKey }}
  -client-key="/consul/connect-inject/tls/tls.key" \
  {{- end }} 
  {{- end }}
  /consul/connect-inject/service.hcl
{{- if .AuthMethod }}
&& /consul/connect-inject/consul logout \
{{- if .EnvoyCertVolume }}
{{- if .EnvoyCAFile }}
  -ca-file="/consul/connect-inject/tls/ca.crt" \
{{- end }}
{{- if .EnvoyClientCert }}
  -client-cert="/consul/connect-inject/tls/tls.crt" \
{{- end }} 
{{- if .EnvoyClientKey }}
  -client-key="/consul/connect-inject/tls/tls.key" \
{{- end }} 
{{- end }}
  -token-file="/consul/connect-inject/acl-token"
{{- end}}
`
