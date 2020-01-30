package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/go-logr/logr"
	_ "github.com/go-sql-driver/mysql"
	mysqlv1alpha1 "github.com/kdichalas/mysql-manager-operator/pkg/apis/mysql/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_mysql")

// Add creates a new Mysql Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMysql{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("mysql-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Mysql
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.Mysql{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMysql implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMysql{}

const mysqlFinalizer = "finalizer.mysql.kdichalas.net"

// ReconcileMysql reconciles a Mysql object
type ReconcileMysql struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Mysql object and makes changes based on the state read
// and what is in the Mysql.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMysql) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Mysql")

	// Fetch the Mysql instance
	instance := &mysqlv1alpha1.Mysql{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
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

	backendCreds := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: instance.Spec.Backend.Credentials.Namespace,
		Name: instance.Spec.Backend.Credentials.Name}, backendCreds)

	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		r.setStage(reqLogger, instance, err.Error())
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	reqLogger.Info("Connecting to database")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema",
		backendCreds.Data["user"],
		backendCreds.Data["password"],
		instance.Spec.Backend.Host,
		instance.Spec.Backend.Port)
	r.setStage(reqLogger, instance, "Connecting to database")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		r.setStage(reqLogger, instance, err.Error())
		return reconcile.Result{}, err
	}
	defer db.Close()

	// Creates or alters a database in a remote mysql instance
	err = handleDatabase(reqLogger, db, instance.Name, instance.Spec.Database.CharacterSet, instance.Spec.Database.Collate)
	if err != nil {
		r.setStage(reqLogger, instance, err.Error())
		return reconcile.Result{}, err
	}

	userCreds := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: instance.Spec.Database.Credentials.Namespace,
		Name: instance.Spec.Database.Credentials.Name,
	}, userCreds)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		r.setStage(reqLogger, instance, err.Error())
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Creates or alters a user in a remote mysql instance
	err = handleUser(reqLogger,
		db,
		instance.Name,
		string(userCreds.Data["user"]),
		string(userCreds.Data["password"]),
		instance.Spec.Database.Host,
		instance.Spec.Database.Privileges)

	if err != nil {
		r.setStage(reqLogger, instance, err.Error())
		return reconcile.Result{}, err
	}


	r.setStage(reqLogger, instance, "Completed")

	// Check if the Mysql instance is marked to be deleted, which is indicated by the deletion timestamp being set.
	isMysqlMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isMysqlMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), mysqlFinalizer) {
			// Run finalization logic for mysqlFinalizer. If the finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeMysql(reqLogger, string(userCreds.Data["user"]), instance.Spec.Database.Host, instance, db); err != nil {
				r.setStage(reqLogger, instance, err.Error())
				return reconcile.Result{}, err
			}

			// Remove mysqlFinalizer. Once all finalizers have been removed, the object will be deleted.
			instance.SetFinalizers(remove(instance.GetFinalizers(), mysqlFinalizer))
			err := r.client.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Check if the Mysql instance contains the mysqlFinalizer and add it
	if !contains(instance.GetFinalizers(), mysqlFinalizer) {
		if err := r.addFinalizer(reqLogger, instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// Actions to perform before the mysql cr deletion
func (r *ReconcileMysql) finalizeMysql(reqLogger logr.Logger, user string, from string, m *mysqlv1alpha1.Mysql, db *sql.DB) error {
	err := dropUser(reqLogger, db, user, from)
	if err != nil {
		return err
	}
	err = dropDatabase(reqLogger, db, m.Name)
	if err != nil {
		return err
	}
	reqLogger.Info("Successfully finalized mysql")
	return nil
}

// Add mysqlFinalizer to the mysql instance
func (r *ReconcileMysql) addFinalizer(reqLogger logr.Logger, m *mysqlv1alpha1.Mysql) error {
	reqLogger.Info("Adding Finalizer for the Mysql")
	m.SetFinalizers(append(m.GetFinalizers(), mysqlFinalizer))

	// Update CR
	err := r.client.Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update Mysql with finalizer")
		return err
	}
	return nil
}

func (r *ReconcileMysql) setStage(reqLogger logr.Logger, m *mysqlv1alpha1.Mysql, stage string) error {
	reqLogger.Info(fmt.Sprintf("Set stage %s", stage))
	m.Status.Stage = stage
	err := r.client.Status().Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update Mysql status")
		return err
	}
	return nil
}