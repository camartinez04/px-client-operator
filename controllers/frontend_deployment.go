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

// deploymentForFrontend returns a Frontend Deployment object
func (r *FrontendReconciler) deploymentForFrontend(Frontend *pxclientv1alpha1.Frontend) (*appsv1.Deployment, error) {

	userid := int64(1000)
	groupid := int64(2000)

	// Labels
	labelsFrontend := labelsForFrontend(Frontend.Name)

	// Replicas
	replicas := Frontend.Spec.Size

	// Label Selector Requirements
	LabelSelectorRequirementVar := metav1.LabelSelectorRequirement{
		Key:      "app.kubernetes.io/name",
		Operator: "In",
		Values:   []string{"Frontend"},
	}

	// Pod Affinity definition
	PodAffinityTermVar := corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				LabelSelectorRequirementVar,
			},
		},
		TopologyKey: "kubernetes.io/hostname",
	}

	// Pod Anti Affinity
	AffinityVar := corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				PodAffinityTermVar,
			},
		},
	}

	// Keycloak url
	keycloakURL := corev1.SecretKeySelector{
		Key: "keycloakUrl",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "keycloak",
		},
	}

	// Keycloak client id
	keycloakClientID := corev1.SecretKeySelector{
		Key: "keycloakClientID",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "keycloak",
		},
	}

	// Keycloak secret
	keycloakSecret := corev1.SecretKeySelector{
		Key: "keycloakSecret",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "keycloak",
		},
	}

	// Keycloak realm
	keycloakRealm := corev1.SecretKeySelector{
		Key: "keycloakRealm",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "keycloak",
		},
	}

	// Environment variables for DB connection
	envVariables := []corev1.EnvVar{
		{
			Name:  "BROKER_URL",
			Value: "http://broker-svc:8081",
		},
		{
			Name: "KEYCLOAK_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &keycloakURL,
			},
		},
		{
			Name: "KEYCLOAK_CLIENT_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &keycloakClientID,
			},
		},
		{
			Name: "KEYCLOAK_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &keycloakSecret,
			},
		},
		{
			Name: "KEYCLOAK_REALM",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &keycloakRealm,
			},
		},
	}

	// Probes for the container, liveness and readiness HTTPGet probe
	containerProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/ping",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 8082,
				},
			},
		},
		InitialDelaySeconds: 7,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    6,
	}

	// Define the main containers for the deployment
	mainContainers := []corev1.Container{{
		Image:           Frontend.Spec.ContainerImage,
		Name:            "frontend",
		ImagePullPolicy: corev1.PullAlways,
		Env:             envVariables,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8082,
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("10Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		ReadinessProbe: &containerProbe,
		LivenessProbe:  &containerProbe,
	}}

	// Define a PodTemplateSpec object
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labelsFrontend,
		},
		Spec: corev1.PodSpec{
			Affinity: &AffinityVar,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  &userid,
				RunAsGroup: &groupid,
			},
			Containers: mainContainers,
		}}

	// Finally, define the Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Frontend.Name,
			Namespace: Frontend.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsFrontend,
			},
			Template: podTemplate,
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(Frontend, deployment, r.Scheme); err != nil {
		return nil, err
	}
	return deployment, nil
}

// serviceForFrontend returns a Frontend Service object
func (r *FrontendReconciler) serviceForFrontend(Frontend *pxclientv1alpha1.Frontend) (serviceFrontend *corev1.Service, err error) {
	// Define the Service
	serviceFrontend = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Frontend.Name + "-svc",
			Namespace: Frontend.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "frontend",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8082,
					TargetPort: intstr.FromInt(8082),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	return serviceFrontend, nil
}
