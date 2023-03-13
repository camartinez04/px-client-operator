package controllers

import (
	pxclientv1alpha1 "github.com/camartinez04/px-client-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// statefulSetForPostgres returns a Postgres StatefulSet object
func (r *PostgresReconciler) statefulSetForPostgres(Postgres *pxclientv1alpha1.Postgres) (*appsv1.StatefulSet, error) {

	userid := int64(0)
	groupid := int64(2000)

	labelsPostgres := labelsForPostgres(Postgres.Name)

	replicas := Postgres.Spec.Size

	// DB password for the DB connection from a Kubernetes secret
	PasswordSecret := corev1.SecretKeySelector{
		Key: "postgresql-password",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "postgres-secrets",
		},
	}

	// Resources

	// Environment variables for DB connection
	envVariables := []corev1.EnvVar{
		{
			Name:  "POSTGRES_DB",
			Value: Postgres.Spec.PostgresDatabase,
		},
		{
			Name:  "POSTGRES_USER",
			Value: Postgres.Spec.PostgresUser,
		},
		{
			Name:  "PGDATA",
			Value: "/var/lib/postgresql/data",
		},

		{
			Name: "POSTGRES_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &PasswordSecret,
			},
		},
	}

	// Probes for the container, liveness and readiness, using HTTPGetAction
	containerProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(Postgres.Spec.ContainerPort)),
			},
		},
		InitialDelaySeconds: 15,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    6,
	}

	// Postgres container definition
	mainContainers := []corev1.Container{
		{
			Image:           Postgres.Spec.ContainerImage,
			Name:            "postgres",
			ImagePullPolicy: corev1.PullAlways,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: Postgres.Spec.ContainerPort,
					Name:          "postgres",
				},
			},
			Env: envVariables,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "postgres-persistent-storage",
					MountPath: "/var/lib/postgresql/data",
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("30m"),
					corev1.ResourceMemory: resource.MustParse("30Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("300Mi"),
				},
			},
			LivenessProbe:  &containerProbe,
			ReadinessProbe: &containerProbe,
		},
	}

	// PVC Spec template
	pvcSpecTemplate := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
		StorageClassName: &Postgres.Spec.StorageClassName,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(Postgres.Spec.StorageSize),
			},
		},
	}

	// StatefulSet template
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Postgres.Name,
			Namespace: Postgres.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsPostgres,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsPostgres,
				},
				Spec: corev1.PodSpec{
					SchedulerName: "stork",
					Containers:    mainContainers,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  &userid,
						RunAsGroup: &groupid,
						FSGroup:    &groupid,
						FSGroupChangePolicy: &[]corev1.PodFSGroupChangePolicy{
							corev1.FSGroupChangeOnRootMismatch,
						}[0],
						SupplementalGroups: []int64{0},
					},
				},
			},
			Replicas: &replicas,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "postgres-persistent-storage",
				},
				Spec: pvcSpecTemplate,
			},
			},
			ServiceName: "postgres-svc",
		},
	}

	// Set the ownerRef for the StatefulSet
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(Postgres, statefulSet, r.Scheme); err != nil {
		return nil, err
	}
	return statefulSet, nil
}

func (r *PostgresReconciler) secretUserForPostgres(Postgres *pxclientv1alpha1.Postgres) (secretPostgres *corev1.Secret, err error) {
	ls := labelsForPostgres(Postgres.Name)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-secrets",
			Namespace: Postgres.Namespace,
			Labels:    ls,
		},
		StringData: map[string]string{
			"postgresql-password": Postgres.Spec.PostgresPassword,
		},
	}

	// Set the ownerRef for the Secret
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(Postgres, secret, r.Scheme); err != nil {
		return nil, err
	}
	return secret, nil
}

func (r *PostgresReconciler) serviceForPostgres(Postgres *pxclientv1alpha1.Postgres) (servicePostgres *corev1.Service, err error) {
	// Define the Service
	servicePostgres = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Postgres.Name + "-svc",
			Namespace: Postgres.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "postgres",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Port:       Postgres.Spec.ContainerPort,
					TargetPort: intstr.FromInt(int(Postgres.Spec.ContainerPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	return servicePostgres, nil
}
