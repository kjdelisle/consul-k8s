package connectinject

import (
	corev1 "k8s.io/api/core/v1"
)

// volumeName is the name of the volume that is created to store the
// Consul Connect injection data.
const volumeName = "consul-connect-inject-data"

// containerVolume returns the volume data to add to the pod. This volume
// is used for shared data between containers.
func (h *Handler) containerVolume() corev1.Volume {
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// containerVolume returns the volume data to add to the pod. This volume
// is used for shared data between containers.
func (h *Handler) envoySecretVolume(volName, caFile, certFile, keyFile string) corev1.Volume {

	items := []corev1.KeyToPath{}
	vol := corev1.Volume{
		Name: volName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: volName,
				Items:      items,
			},
		},
	}

	if caFile != "" {
		items = append(items, corev1.KeyToPath{
			Key:  caFile,
			Path: "/consul/connect-inject/ca.crt",
		})
	}

	if certFile != "" {
		items = append(items, corev1.KeyToPath{
			Key:  certFile,
			Path: "/consul/connect-inject/tls.crt",
		})
	}

	if keyFile != "" {
		items = append(items, corev1.KeyToPath{
			Key:  keyFile,
			Path: "/consul/connect-inject/tls.key",
		})
	}

	return vol
}
