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

// deploymentForBroker returns a Broker Deployment object
func (r *BrokerReconciler) deploymentForBroker(Broker *pxclientv1alpha1.Broker) (*appsv1.Deployment, error) {

	userid := int64(1000)
	groupid := int64(2000)

	// Labels
	labelsBroker := labelsForBroker(Broker.Name)

	// Replicas
	replicas := Broker.Spec.Size

	// Label Selector Requirements
	LabelSelectorRequirementVar := metav1.LabelSelectorRequirement{
		Key:      "app.kubernetes.io/name",
		Operator: "In",
		Values:   []string{"Broker"},
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
			Name:  "PORTWORX_GRPC_URL",
			Value: "portworx-service.kube-system:9020",
		},
		{
			Name:  "PORTWORX_TOKEN",
			Value: Broker.Spec.PortworxToken,
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

	// Probes for the container, liveness and readiness, HTTPGet probe
	containerProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/ping",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 8081,
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
		Image:           Broker.Spec.ContainerImage,
		Name:            "broker",
		ImagePullPolicy: corev1.PullAlways,
		Env:             envVariables,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8081,
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
			Labels: labelsBroker,
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
			Name:      Broker.Name,
			Namespace: Broker.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsBroker,
			},
			Template: podTemplate,
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(Broker, deployment, r.Scheme); err != nil {
		return nil, err
	}
	return deployment, nil
}

// serviceForBroker returns a Broker Service object
func (r *BrokerReconciler) serviceForBroker(Broker *pxclientv1alpha1.Broker) (serviceBroker *corev1.Service, err error) {
	// Define the Service
	serviceBroker = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Broker.Name + "-svc",
			Namespace: Broker.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "broker",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8081,
					TargetPort: intstr.FromInt(8081),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	return serviceBroker, nil
}
