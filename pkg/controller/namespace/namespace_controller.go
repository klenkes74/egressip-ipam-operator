package namespace

import (
	"context"
	"reflect"

	ocpnetv1 "github.com/openshift/api/network/v1"
	"github.com/redhat-cop/egressip-ipam-operator/pkg/controller/egressipam"
	"github.com/redhat-cop/operator-utils/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "namespace-controller"

var log = logf.Log.WithName("controllerName")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Namespace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNamespace{
		ReconcilerBase: util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName)),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	IsAnnotated := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, okold := e.MetaOld.GetAnnotations()[egressipam.NamespaceAnnotation]
			_, oknew := e.MetaNew.GetAnnotations()[egressipam.NamespaceAnnotation]
			return (okold && !oknew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	// Watch for changes to primary resource Namespace
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{}, IsAnnotated)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileNamespace implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNamespace{}

// ReconcileNamespace reconciles a Namespace object
type ReconcileNamespace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	util.ReconcilerBase
}

// Reconcile reads that state of the cluster for a Namespace object and makes changes based on the state read
// and what is in the Namespace.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Namespace")

	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	err = r.cleanUpNamespaceAndNetNamespace(instance)
	if err != nil {
		log.Error(err, "unable to clean up", "netnamespace", instance.GetName())
		return r.ManageError(instance, err)
	}

	return r.ManageSuccess(instance)
}

func (r *ReconcileNamespace) cleanUpNamespaceAndNetNamespace(namespace *corev1.Namespace) error {
	netNamespace := &ocpnetv1.NetNamespace{}
	err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: namespace.GetName()}, netNamespace)
	if err != nil {
		log.Error(err, "unable to retrieve", "netnamespace", namespace.GetName())
		return err
	}
	if !reflect.DeepEqual(netNamespace.EgressIPs, []string{}) {
		netNamespace.EgressIPs = []ocpnetv1.NetNamespaceEgressIP{}
		err := r.GetClient().Update(context.TODO(), netNamespace, &client.UpdateOptions{})
		if err != nil {
			log.Error(err, "unable to update ", "netnamespace", netNamespace.GetName())
			return err
		}
	}
	if _, ok := namespace.GetAnnotations()[egressipam.NamespaceAssociationAnnotation]; ok {
		delete(namespace.Annotations, egressipam.NamespaceAssociationAnnotation)
		err := r.GetClient().Update(context.TODO(), namespace, &client.UpdateOptions{})
		if err != nil {
			log.Error(err, "unable to update ", "namespace", namespace.GetName())
			return err
		}
	}
	return nil
}
