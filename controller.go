package main

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	// not sure
	"k8s.io/apimachinery/pkg/fields"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ec2"
)

const (
	controllerAgentName = "kube-sg-controller"
	resyncPeriod = 15 * time.Minute
	maxRetries = 5
)

type Controller struct {
	kubeclientset kubernetes.Interface
	informer      cache.SharedIndexInformer
	queue         workqueue.RateLimitingInterface
	recorder      record.EventRecorder
	clusterName   string
	region        string
	//eventHandler  handlers.Handler
	//podLister lister_v1.PodLister
	//podController       *cache.Controller
	//informer   *cache.Controller
}

type Task struct {
	Key        string
	CidrIp     string
	Annotation string
}

func NewController(kubeclientset kubernetes.Interface, clusterName string) *Controller {

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	listwatch := cache.NewListWatchFromClient(kubeclientset.CoreV1().RESTClient(), "pods", metav1.NamespaceAll, fields.Everything())

	informer := cache.NewSharedIndexInformer(
		listwatch,
		&corev1.Pod{},
		resyncPeriod,
		cache.Indexers{},
		//cache.Indexers{podIPIndexName: kube2iam.PodIPIndexFunc},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(Task{
					key,
					obj.(*corev1.Pod).Status.PodIP + "/32",
					obj.(*corev1.Pod).Annotations["sg.amazonaws.com/ingress"],
				})
			} else {
				runtime.HandleError(err)
				return
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.AddRateLimited(Task{
					key,
					new.(*corev1.Pod).Status.PodIP + "/32",
					new.(*corev1.Pod).Annotations["sg.amazonaws.com/ingress"],
				})
			} else {
				runtime.HandleError(err)
				return
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(Task{
					key,
					obj.(*corev1.Pod).Status.PodIP + "/32",
					obj.(*corev1.Pod).Annotations["sg.amazonaws.com/ingress"],
				})
			} else {
				runtime.HandleError(err)
				return
			}
		},
	})

	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset: kubeclientset,
		informer:      informer,
		queue:         queue,
		recorder:      recorder,
		clusterName:   clusterName,
		region:        getRegion(),
	}

  //controller.informer = informer
	//controller.podLister = lister_v1.NewPodLister(indexer)
	//controller.recorder = recorder


	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting kube-sg controller")

	go c.informer.Run(stopCh)

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.informer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNext() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNext() bool {
	key, quit := c.queue.Get()

	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.process(key.(Task))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxRetries {
		glog.Infof("Error processing %s (will retry): %v", key, err)
		c.queue.AddRateLimited(key)
	} else {
		// err != nil and too many retries
		glog.Errorf("Error processing %s (giving up): %v", key, err)
		c.queue.Forget(key)
		runtime.HandleError(err)
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) process(key Task) error {

	description := c.clusterName + "/" + key.Key
	rules := fromAnnotationsToRules(key.Annotation, key.CidrIp, description)

	if len(rules) == 0 {
		return nil
	}

	obj, exists, err := c.informer.GetIndexer().GetByKey(key.Key)
	if err != nil {
		return fmt.Errorf("failed to retrieve pod by key %q: %v", key.Key, err)
	}

	sess := session.New(&aws.Config{Region: aws.String(c.region)})
	svc := ec2.New(sess)

	if !exists {

		for id := range rules {
			_, err = svc.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		        GroupId: aws.String(id),
		        IpPermissions: rules[id],
		    })
				if err != nil {
					glog.Info(err)
				}
		}

		glog.Infof("security groups ingress rules deleted for '%s': '%s', pod IP: %s", key.Key, key.Annotation, key.CidrIp)
		return nil
	}

	for id := range rules {
		_, err = svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(id),
	    IpPermissions: rules[id],
	  })
		if err != nil {
			glog.Info(err)
		}
	}

	glog.Infof("security groups ingress rules created for '%s': '%s', pod IP: %s", key.Key, key.Annotation, key.CidrIp)
	c.recorder.Eventf(obj.(*corev1.Pod), corev1.EventTypeNormal, "pod created", "security groups ingress rules created: %s", key.Annotation)
	return nil

}
