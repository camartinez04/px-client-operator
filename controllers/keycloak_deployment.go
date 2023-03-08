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

// deploymentForKeycloak returns a Keycloak Deployment object
func (r *KeycloakReconciler) deploymentForKeycloak(Keycloak *pxclientv1alpha1.Keycloak) (*appsv1.Deployment, error) {

	userid := int64(1000)
	groupid := int64(2000)

	// Labels
	labelsKeycloak := labelsForKeycloak(Keycloak.Name)

	// Replicas
	replicas := Keycloak.Spec.Size

	// Label Selector Requirements
	LabelSelectorRequirementVar := metav1.LabelSelectorRequirement{
		Key:      "app.kubernetes.io/name",
		Operator: "In",
		Values:   []string{"keycloak"},
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

	// Environment variables for DB connection
	envVariables := []corev1.EnvVar{
		{
			Name:  "KEYCLOAK_ADMIN",
			Value: "admin",
		},
		{
			Name:  "KEYCLOAK_ADMIN_PASSWORD",
			Value: "change_me",
		},
		{
			Name:  "KC_DB_URL",
			Value: "jdbc:postgresql://postgres-svc/keycloak",
		},
		{
			Name:  "KC_DB_USERNAME",
			Value: "postgres",
		},
		{
			Name:  "KC_DB_PASSWORD",
			Value: "postgres",
		},
		{
			Name:  "KC_HOSTNAME_STRICT",
			Value: "false",
		},
		{
			Name:  "KC_HTTP_ENABLED",
			Value: "true",
		},
		{
			Name:  "PROXY_ADDRESS_FORWARDING",
			Value: "true",
		},
		{
			Name:  "KC_HOSTNAME_ADMIN_URL",
			Value: "http://localhost:8080/auth",
		},
		{
			Name:  "KC_HOSTNAME_URL",
			Value: "http://localhost:8080/auth",
		},
	}

	// Define the main containers for the deployment
	mainContainers := []corev1.Container{{
		Image:           "calvarado2004/portworx-client-keycloak:latest",
		Name:            "keycloak",
		ImagePullPolicy: corev1.PullAlways,
		Env:             envVariables,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
			},
			{
				ContainerPort: 8443,
				Name:          "https",
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1000m"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
	}}

	// Define a PodTemplateSpec object
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labelsKeycloak,
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
			Name:      Keycloak.Name,
			Namespace: Keycloak.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsKeycloak,
			},
			Template: podTemplate,
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(Keycloak, deployment, r.Scheme); err != nil {
		return nil, err
	}
	return deployment, nil
}

// serviceForKeycloak returns a Keycloak Service object
func (r *KeycloakReconciler) serviceForKeycloak(Keycloak *pxclientv1alpha1.Keycloak) (serviceKeycloak *corev1.Service, err error) {
	// Define the Service
	serviceKeycloak = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Keycloak.Name + "-svc",
			Namespace: Keycloak.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "keycloak",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "https",
					Port:       8443,
					TargetPort: intstr.FromInt(8443),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	return serviceKeycloak, nil
}

// secretKeycloakRealm returns a Keycloak Realm Secret object
func (r *KeycloakReconciler) secretKeycloakRealm(Keycloak *pxclientv1alpha1.Keycloak) (secretKeycloak *corev1.Secret, err error) {
	ls := labelsForPostgres(Keycloak.Name)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak",
			Namespace: Keycloak.Namespace,
			Labels:    ls,
		},
		StringData: map[string]string{
			"keycloakClientID": "cG9ydHdvcngtY2xpZW50",
			"keycloakRealm":    "cG9ydHdvcng=",
			"keycloakSecret":   "cjdaYndzcEJUNTZwUDVCNWNNTlNZd3l3S0l1dzN5U3M=",
			"keycloakUrl":      "aHR0cDovL2tleWNsb2FrOjgwODA=",
		},
	}

	// Set the ownerRef for the Secret
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(Keycloak, secret, r.Scheme); err != nil {
		return nil, err
	}
	return secret, nil
}
