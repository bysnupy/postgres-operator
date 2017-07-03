/*
 Copyright 2017 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/crunchydata/postgres-operator/operator/util"
	"github.com/crunchydata/postgres-operator/tpr"
	"k8s.io/client-go/kubernetes"
	kerrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"os"
	"strings"
	"time"
)

func ProcessPolicies(clientset *kubernetes.Clientset, tprclient *rest.RESTClient, stopchan chan struct{}, namespace string) {

	lo := v1.ListOptions{LabelSelector: "pg-cluster,!replica"}
	fw, err := clientset.Deployments(namespace).Watch(lo)
	if err != nil {
		log.Error("fatal error in ProcessPolicies " + err.Error())
		os.Exit(2)
	}

	_, err4 := watch.Until(0, fw, func(event watch.Event) (bool, error) {
		log.Infof("got a processpolicies watch event %v\n", event.Type)

		switch event.Type {
		case watch.Added:
			//deployment := event.Object.(*v1beta1.Deployment)
			//log.Infof("deployment processpolicy added=%s\n", dep.Name)
		case watch.Deleted:
			//deployment := event.Object.(*v1beta1.Deployment)
			//log.Infof("deployment processpolicy deleted=%s\n", deployment.Name)
		case watch.Error:
			log.Infof("deployment processpolicy error event")
		case watch.Modified:
			deployment := event.Object.(*v1beta1.Deployment)
			//log.Infof("deployment processpolicy modified=%s\n", deployment.Name)
			log.Infof("status available replicas=%d\n", deployment.Status.AvailableReplicas)
			if deployment.Status.AvailableReplicas > 0 {
				applyPolicies(namespace, clientset, tprclient, deployment)
			}
		default:
			log.Infoln("processpolices unknown watch event %v\n", event.Type)
		}

		return false, nil
	})

	if err4 != nil {
		log.Error("error in ProcessPolicies " + err4.Error())
	}

}

func applyPolicies(namespace string, clientset *kubernetes.Clientset, tprclient *rest.RESTClient, dep *v1beta1.Deployment) {
	//get the tpr which holds the requested labels if any
	cl := tpr.PgCluster{}
	err := tprclient.Get().
		Resource("pgclusters").
		Namespace(namespace).
		Name(dep.Name).
		Do().
		Into(&cl)
	if err == nil {
	} else if kerrors.IsNotFound(err) {
		log.Error("could not get cluster in policy processing using " + dep.Name)
		return
	} else {
		log.Error("error in policy processing " + err.Error())
		return
	}

	if cl.Spec.Policies == "" {
		log.Debug("no policies to apply to " + dep.Name)
		return
	}
	log.Debug("policies to apply to " + dep.Name + " are " + cl.Spec.Policies)
	policies := strings.Split(cl.Spec.Policies, ",")

	//apply the policies
	labels := make(map[string]string)

	for _, v := range policies {
		err = util.ExecPolicy(clientset, tprclient, namespace, v, cl.Spec.Name)
		if err != nil {
			log.Error(err)
		} else {
			labels[v] = "pgpolicy"
		}

	}

	//update the deployment's labels to show applied policies
	err = util.UpdateDeploymentLabels(clientset, dep.Name, namespace, labels)
	if err != nil {
		log.Error(err)
	}
}

func ProcessPolicylog(clientset *kubernetes.Clientset, tprclient *rest.RESTClient, stopchan chan struct{}, namespace string) {

	eventchan := make(chan *tpr.PgPolicylog)

	source := cache.NewListWatchFromClient(tprclient, tpr.POLICY_LOG_RESOURCE, namespace, fields.Everything())

	createAddHandler := func(obj interface{}) {
		job := obj.(*tpr.PgPolicylog)
		eventchan <- job
		addPolicylog(clientset, tprclient, job, namespace)
	}
	createDeleteHandler := func(obj interface{}) {
		//job := obj.(*tpr.PgUpgrade)
		//eventchan <- job
		//deleteUpgrade(clientset, client, job, namespace)
	}

	updateHandler := func(old interface{}, obj interface{}) {
		//job := obj.(*tpr.PgUpgrade)
		//eventchan <- job
		//log.Info("updating PgUpgrade object")
		//log.Info("updated with Name=" + job.Spec.Name)
	}
	_, controller := cache.NewInformer(
		source,
		&tpr.PgPolicylog{},
		time.Second*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    createAddHandler,
			UpdateFunc: updateHandler,
			DeleteFunc: createDeleteHandler,
		})

	go controller.Run(stopchan)

	for {
		select {
		case event := <-eventchan:
			//log.Infof("%#v\n", event)
			if event == nil {
				log.Info("event was null")
			}
		}
	}

}

func addPolicylog(clientset *kubernetes.Clientset, tprclient *rest.RESTClient, policylog *tpr.PgPolicylog, namespace string) {
	log.Infof("policylog added=%s\n", policylog.Spec.PolicyName+policylog.Spec.ClusterName)

	labels := make(map[string]string)

	err := util.ExecPolicy(clientset, tprclient, namespace, policylog.Spec.PolicyName, policylog.Spec.ClusterName)
	if err != nil {
		log.Error(err)
	} else {
		labels[policylog.Spec.PolicyName] = "pgpolicy"
	}

	//update the deployment's labels to show applied policies
	err = util.UpdateDeploymentLabels(clientset, policylog.Spec.ClusterName, namespace, labels)
	if err != nil {
		log.Error(err)
	}
}
