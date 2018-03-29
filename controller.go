package main

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	//kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	//lister_v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	//"k8s.io/client-go/pkg/api/v1"

	//samplev1alpha1 "k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1"
	//clientset "k8s.io/sample-controller/pkg/client/clientset/versioned"
	//"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	//informers "k8s.io/sample-controller/pkg/client/informers/externalversions"
	//listers "k8s.io/sample-controller/pkg/client/listers/samplecontroller/v1alpha1"

	// not sure
	"k8s.io/apimachinery/pkg/fields"


	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ec2"
	"strings"
  "strconv"
)

const (
	controllerAgentName = "kube-sg-controller"
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"
	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	resyncPeriod = 15 * time.Minute

	maxRetries = 5
)

type Controller struct {
	kubeclientset kubernetes.Interface
	informer      cache.SharedIndexInformer
	queue     workqueue.RateLimitingInterface
	recorder      record.EventRecorder
	//eventHandler  handlers.Handler
	//podLister lister_v1.PodLister
	//podController       *cache.Controller
	//informer   *cache.Controller
}

type Task struct {
	Annotation string
	CidrIp     string
	Key        string
}

func NewController(kubeclientset kubernetes.Interface) *Controller {

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
				queue.AddRateLimited(Task{obj.(*corev1.Pod).Annotations["sg.amazonaws.com/ingress"],obj.(*corev1.Pod).Status.PodIP + "/32", key})
			} else {
				runtime.HandleError(err)
				return
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.AddRateLimited(Task{new.(*corev1.Pod).Annotations["sg.amazonaws.com/ingress"],new.(*corev1.Pod).Status.PodIP + "/32", key})
			} else {
				runtime.HandleError(err)
				return
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(Task{obj.(*corev1.Pod).Annotations["sg.amazonaws.com/ingress"],obj.(*corev1.Pod).Status.PodIP + "/32", key})
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
		//eventHandler: eventHandler,
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
	obj, exists, err := c.informer.GetIndexer().GetByKey(key.Key)
	if err != nil {
		return fmt.Errorf("failed to retrieve pod by key %q: %v", key.Key, err)
	}

	description := key.Key
	cidrIp := key.CidrIp
	rules := fromAnnotationsToRules(key.Annotation)

	sess := session.New(&aws.Config{Region: aws.String("us-east-1")})
  svc := ec2.New(sess)

	if !exists {

		for _, rule := range rules {
			from, _ := strconv.ParseInt(rule["from"], 10, 64)
			to, _ := strconv.ParseInt(rule["to"], 10, 64)
			_, err = svc.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		        GroupId: aws.String(rule["id"]),
		        IpPermissions: []*ec2.IpPermission{
		            (&ec2.IpPermission{}).
										SetIpProtocol(rule["protocol"]).
										SetFromPort(from).
		                SetToPort(to).
		                SetIpRanges([]*ec2.IpRange{
		                    {
													CidrIp: aws.String(cidrIp),
													Description: aws.String(description),
												},
		                }),
		        },
		    })
				if err != nil {
					glog.Info(err)
				}
		}

		glog.Info("Delete sg")
		return nil
	}

	//cidrIp := obj.(*corev1.Pod).Status.PodIP + "/32"

	for _, rule := range rules {
		from, _ := strconv.ParseInt(rule["from"], 10, 64)
		to, _ := strconv.ParseInt(rule["to"], 10, 64)
		_, err = svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
	        GroupId: aws.String(rule["id"]),
	        IpPermissions: []*ec2.IpPermission{
	            (&ec2.IpPermission{}).
	                SetIpProtocol(rule["protocol"]).
	                SetFromPort(from).
	                SetToPort(to).
	                SetIpRanges([]*ec2.IpRange{
	                    {
												CidrIp: aws.String(cidrIp),
												Description: aws.String(description),
											},
	                }),
	        },
	    })
	}

	glog.Infof("Update or create sg %s %s %s", obj.(*corev1.Pod).Name, obj.(*corev1.Pod).Status.PodIP, key)
	return nil

}

func fromAnnotationsToRules(annotations string) []map[string]string {

	var rules []map[string]string

	for _, rule := range strings.Split(annotations, ",") {
		s := strings.Split(rule, ":")
		if len(s) == 3 {
			ports := strings.Split(s[2], "-")
			if len(ports) == 2 {
				ruleItem := map[string]string{
					"id"      : s[0],
					"protocol": s[1],
					"from":     ports[0],
					"to":       ports[1],
				}
				rules = append(rules, ruleItem)
			} else if len(ports) == 1 {
				ruleItem := map[string]string{
					"id"      : s[0],
					"protocol": s[1],
					"from":     ports[0],
					"to":       ports[0],
				}
				rules = append(rules, ruleItem)
			}
		}
  }

	return rules
}
