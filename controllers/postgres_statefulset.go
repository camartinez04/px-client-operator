package controllers

import (
	"context"
	"fmt"
	pxclientv1alpha1 "github.com/camartinez04/px-client-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// isRunningOnOpenShift returns true if the operator is running on OpenShift, false otherwise.
func (r *PostgresReconciler) isRunningOnOpenShift(ctx context.Context) bool {

	// Check the existence of an OpenShift-specific API resource.
	// E.g., we'll check for 'Project' which is OpenShift specific.
	gvr := schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projects"}

	_, err := r.DynClient.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		// If we get an error and it's due to the resource not existing, assume it's vanilla Kubernetes
		if apierrors.IsNotFound(err) {
			return false
		}
		// Otherwise, it's an unexpected error, log it and assume it's not OpenShift for safety.
		// (You might want to handle this case differently, depending on your requirements)
		fmt.Println("Unexpected error:", err)
		return false
	}

	// If no error, it's likely OpenShift
	return true
}

// patchDefaultServiceAccountForAnyUID - This function will patch the default service account to allow any UID to run
func (r *PostgresReconciler) patchDefaultServiceAccountForAnyUID(namespace string) error {
	sccToPatch := "anyuid"

	sccGVR := schema.GroupVersionResource{
		Group:    "security.openshift.io",
		Version:  "v1",
		Resource: "securitycontextconstraints",
	}

	// create the user string for the default service account
	userToAdd := "system:serviceaccount:" + namespace + ":default" // <-- Note the 'default' SA

	// get the current SCC to check the users
	sccUnstructured, err := r.DynClient.Resource(sccGVR).Get(context.TODO(), sccToPatch, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// check if the user already exists in the SCC
	users, found, err := unstructured.NestedStringSlice(sccUnstructured.UnstructuredContent(), "users")
	if err != nil {
		return err
	}

	if found {
		for _, user := range users {
			if user == userToAdd {
				// The user is already added; no action needed
				return nil
			}
		}
	}

	// patch the SCC
	patchData := []byte(`[
        {
            "path": "/users/-",
            "op": "add",
            "value": "` + userToAdd + `"
        }
    ]`)

	_, err = r.DynClient.Resource(sccGVR).Patch(context.TODO(), sccToPatch, types.JSONPatchType, patchData, metav1.PatchOptions{})
	return err
}
