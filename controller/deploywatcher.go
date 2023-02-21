package deployWatcher

import (
	"context"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	annotationKey = "apigtw"
	annotationVal = "public"
)

type DeployWatcher struct {
	clientset *kubernetes.Clientset
	queue     workqueue.RateLimitingInterface
	informer  cache.SharedIndexInformer
}

func NewDeployWatcher() *DeployWatcher {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Erroe to configure client: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error to create client: %v", err)
	}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.AppsV1().Deployments("").List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.AppsV1().Deployments("").Watch(context.Background(), options)
			},
		},
		&appsv1.Deployment{},
		0,
		cache.Indexers{},
	)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return &DeployWatcher{
		clientset: clientset,
		queue:     queue,
		informer:  informer,
	}
}

func (c *DeployWatcher) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		log.Fatal("Fail to cache sync")
	}

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				log.Printf("Error to get key: %v", err)
				return
			}
			c.queue.Add(key)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err != nil {
				log.Printf("Error to get key: %v", err)
				return
			}
			c.queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {},
	})

	for i := 0; i < 2; i++ {
		go c.runWorker()
	}

	<-stopCh
}

func (c *DeployWatcher) runWorker() {
	for c.processNextItem() {
	}
}

func (c *DeployWatcher) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	obj, exists, err := c.informer.GetIndexer().GetByKey(key.(string))
	if err != nil {
		log.Printf("Error to get Kubernetes object: %v", err)
		return true
	}
	if !exists {
		return true
	}

	deploy, ok := obj.(*appsv1.Deployment)
	if !ok {
		log.Printf("Not is a Deployment: %v", obj)
		return true
	}

	annotations := deploy.GetAnnotations()
	apigtw, ok := annotations[annotationKey]
	if !ok {
		log.Printf("Annotation \"%s\" NOT FOUNT on Deployment %s/%s", annotationKey, deploy.Namespace, deploy.Name)
		return true
	}

	if apigtw != annotationVal {
		log.Printf("Annotation \"%s\" not is \"%s\" on Deployment %s/%s", annotationKey, annotationVal, deploy.Namespace, deploy.Name)
		return true
	}

	log.Printf("Exits Annotation \"apigtw\" EQUALS %s Deployment %s/%s", annotationVal, deploy.Namespace, deploy.Name)

	return true
}
