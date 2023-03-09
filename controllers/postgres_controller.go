package controllers

import (
	"context"
	"fmt"
	pxclientv1alpha1 "github.com/camartinez04/px-client-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

const postgresFinalizer = "pxclient.calvarado04.com/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailablePostgres represents the status of the Deployment reconciliation
	typeAvailablePostgres = "Available"
	// typeDegradedPostgres represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedPostgres = "Degraded"
)

// PostgresReconciler reconciles a Postgres object
type PostgresReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// The following markers are used to generate the rules permissions (RBAC) on config/rbac using controller-gen
// when the command <make manifests> is executed.
// To know more about markers see: https://book.kubebuilder.io/reference/markers.html

//+kubebuilder:rbac:groups=pxclient.calvarado04.com,resources=postgres,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pxclient.calvarado04.com,resources=postgres/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pxclient.calvarado04.com,resources=postgres/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods;services;events;secrets;configmaps;persistentvolumeclaims,verbs=create;patch;update;delete;get;list;watch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *PostgresReconciler) CreateSecret(ctx context.Context, Postgres *pxclientv1alpha1.Postgres) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if Postgres secret exists, if not create it new
	secretFound := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: "postgres-secrets", Namespace: Postgres.Namespace}, secretFound)
	if err != nil && apierrors.IsNotFound(err) {

		secret, err := r.secretUserForPostgres(Postgres)
		if err != nil {
			log.Error(err, "Failed to create Secret resource for Postgres")

			// The following implementation will update the status
			meta.SetStatusCondition(&Postgres.Status.Conditions, metav1.Condition{Type: typeAvailablePostgres,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Secret for the custom resource (%s): (%s)", Postgres.Name, err)})

			if err := r.Status().Update(ctx, Postgres); err != nil {
				log.Error(err, "Failed to update Postgres status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		if err = r.Create(ctx, secret); err != nil && apierrors.IsNotFound(err) {
			log.Error(err, "Failed to create secret for Postgres on namespace")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		log.Info("Secret for Postgres was created successfully", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)

		// Secret created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil

	} else if err != nil {
		log.Error(err, "Failed to get Secret")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil

}

func (r *PostgresReconciler) CreateService(ctx context.Context, Postgres *pxclientv1alpha1.Postgres) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if Postgres service exists, if not create it new
	serviceFound := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: "postgres-svc", Namespace: Postgres.Namespace}, serviceFound)
	if err != nil && apierrors.IsNotFound(err) {
		svc, err := r.serviceForPostgres(Postgres)
		if err != nil {
			log.Error(err, "Failed to define new Service resource for Postgres")

			// The following implementation will update the status
			meta.SetStatusCondition(&Postgres.Status.Conditions, metav1.Condition{Type: typeAvailablePostgres,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Service for the custom resource (%s): (%s)", Postgres.Name, err)})

			if err := r.Status().Update(ctx, Postgres); err != nil {
				log.Error(err, "Failed to update Postgres status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		if err = r.Create(ctx, svc); err != nil && apierrors.IsNotFound(err) {
			log.Error(err, "Failed to create new Service",
				"Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		// Service created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil

	} else if err != nil {
		log.Error(err, "Failed to get Service")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil

}

func (r *PostgresReconciler) CreateStatefulSet(ctx context.Context, Postgres *pxclientv1alpha1.Postgres) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	// Check if the StatefulSet already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: Postgres.Name, Namespace: Postgres.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {

		// Define a new StatefulSet
		sts, err := r.statefulSetForPostgres(Postgres)
		if err != nil {
			log.Error(err, "Failed to define new StatefulSet resource for Postgres")

			// The following implementation will update the status
			meta.SetStatusCondition(&Postgres.Status.Conditions, metav1.Condition{Type: typeAvailablePostgres,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", Postgres.Name, err)})

			if err := r.Status().Update(ctx, Postgres); err != nil {
				log.Error(err, "Failed to update Postgres status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new StatefulSet",
			"StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
		if err = r.Create(ctx, sts); err != nil && apierrors.IsNotFound(err) {
			log.Error(err, "Failed to create new StatefulSet for Postgres",
				"StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		// StatefulSet created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil

	} else if err != nil {
		log.Error(err, "Failed to get StatefulSet")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil

}

func (r *PostgresReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Postgres instance
	// The purpose is check if the Custom Resource for the Kind Postgres
	// is applied on the cluster if not we return nil to stop the reconciliation
	Postgres := &pxclientv1alpha1.Postgres{}
	err := r.Get(ctx, req.NamespacedName, Postgres)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Postgres resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Postgres")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if Postgres.Status.Conditions == nil || len(Postgres.Status.Conditions) == 0 {
		meta.SetStatusCondition(&Postgres.Status.Conditions, metav1.Condition{Type: typeAvailablePostgres, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, Postgres); err != nil {
			log.Error(err, "Failed to update Postgres status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the Postgres Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, Postgres); err != nil {
			log.Error(err, "Failed to re-fetch Postgres")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(Postgres, postgresFinalizer) {
		log.Info("Adding Finalizer for Postgres")
		if ok := controllerutil.AddFinalizer(Postgres, postgresFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, Postgres); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the Postgres instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPostgresMarkedToBeDeleted := Postgres.GetDeletionTimestamp() != nil
	if isPostgresMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(Postgres, postgresFinalizer) {
			log.Info("Performing Finalizer Operations for Postgres before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&Postgres.Status.Conditions, metav1.Condition{Type: typeDegradedPostgres,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", Postgres.Name)})

			if err := r.Status().Update(ctx, Postgres); err != nil {
				log.Error(err, "Failed to update Postgres status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForPostgres(Postgres)

			if err := r.Get(ctx, req.NamespacedName, Postgres); err != nil {
				log.Error(err, "Failed to re-fetch Postgres")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&Postgres.Status.Conditions, metav1.Condition{Type: typeDegradedPostgres,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", Postgres.Name)})

			if err := r.Status().Update(ctx, Postgres); err != nil {
				log.Error(err, "Failed to update Postgres status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for Postgres after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(Postgres, postgresFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for Postgres")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, Postgres); err != nil {
				log.Error(err, "Failed to remove finalizer for Postgres")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	_, err = r.CreateSecret(ctx, Postgres)
	if err != nil {
		log.Error(err, "Failed to create Secret")
		return ctrl.Result{}, err
	}

	_, err = r.CreateService(ctx, Postgres)
	if err != nil {
		log.Error(err, "Failed to create Service")
		return ctrl.Result{}, err
	}

	_, err = r.CreateStatefulSet(ctx, Postgres)
	if err != nil {
		log.Error(err, "Failed to create StatefulSet")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil

}

// doFinalizerOperationsForPostgres will perform the required operations before delete the CR.
func (r *PostgresReconciler) doFinalizerOperationsForPostgres(cr *pxclientv1alpha1.Postgres) {

	log := log.FromContext(context.Background())

	// The following implementation will raise an event
	//r.Recorder.Event(cr, "Warning", "Deleting",
	//	fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
	//		cr.Name,
	//		cr.Namespace))

	ok := controllerutil.RemoveFinalizer(cr, postgresFinalizer)
	if ok {
		if err := r.Update(context.Background(), cr); err != nil {
			log.Error(err, "Failed to remove finalizer from Postgres")
			return
		}
	}

	log.Info("Finalizer has been removed from Postgres")

	// Let's delete the PVC that was created for the Postgres StatefulSet
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-persistent-storage-postgres-0",
			Namespace: cr.Namespace,
		},
	}

	err := r.Delete(context.Background(), &pvc, client.PropagationPolicy(metav1.DeletePropagationForeground))
	if err != nil {
		log.Error(err, "Failed to delete PVC")
		return
	}

	log.Info("Postgres PVC was successfully deleted")

	// Let's delete the Service that was created for the Postgres StatefulSet
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-svc",
			Namespace: cr.Namespace,
		},
	}

	err = r.Delete(context.Background(), &service, client.PropagationPolicy(metav1.DeletePropagationForeground))
	if err != nil {
		log.Error(err, "Failed to delete postgres service")
		return
	}

	log.Info("Postgres service was successfully deleted")

}

// labelsForPostgres returns the labels for selecting the resources for the Postgres StatefulSet
func labelsForPostgres(name string) map[string]string {
	var imageTag string
	image, err := imageForPostgres()
	if err == nil {
		imageTag = strings.Split(image, ":")[1]
	}
	return map[string]string{"app.kubernetes.io/name": "postgres",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "Postgres-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// imageForPostgres gets the Postgres image which is managed by this controller
func imageForPostgres() (image string, errorFound error) {
	var imageEnvVar = "POSTGRES_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("Unable to find %s environment variable with the image", imageEnvVar)
	}
	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
// Note that the Deployment will be also watched in order to ensure its
// desirable state on the cluster
func (r *PostgresReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pxclientv1alpha1.Postgres{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
