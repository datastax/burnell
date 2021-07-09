//
//  Copyright (c) 2021 Datastax, Inc.
//
//  Licensed to the Apache Software Foundation (ASF) under one
//  or more contributor license agreements.  See the NOTICE file
//  distributed with this work for additional information
//  regarding copyright ownership.  The ASF licenses this file
//  to you under the Apache License, Version 2.0 (the
//  "License"); you may not use this file except in compliance
//  with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an
//  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
//  KIND, either express or implied.  See the License for the
//  specific language governing permissions and limitations
//  under the License.
//

package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apex/log"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Client is the k8s client object
type Client struct {
	Clientset        *kubernetes.Clientset
	Metrics          *metrics.Clientset
	ClusterName      string
	DefaultNamespace string
}

// Deployment is the k8s deployment
type Deployment struct {
	Name      string
	Replicas  int32
	Instances int32
}

// StatefulSet is the k8s sts
type StatefulSet struct {
	Name      string
	Replicas  int32
	Instances int32
}

// LocalClient is the global k8s client object
var LocalClient *Client

// Init initializes kubernetes access configuration
func Init() {
	var err error
	LocalClient, err = GetK8sClient()
	if err != nil {
		panic(fmt.Errorf("failed to get k8s clientset %v or get pods under pulsar namespace", err))
	}
	log.Infof("k8s clientset initialized")
}

// GetK8sClient gets k8s clientset
func GetK8sClient() (*Client, error) {
	var config *rest.Config

	if home := homedir.HomeDir(); home != "" {
		// TODO: add configuration to allow customized config file
		kubeconfig := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
			log.Infof("this is an in-cluster k8s monitor")
			if config, err = rest.InClusterConfig(); err != nil {
				return nil, err
			}

		} else {
			log.Infof("this is outside of k8s cluster deployment, kubeconfig dir %s", kubeconfig)
			if config, err = clientcmd.BuildConfigFromFlags("", kubeconfig); err != nil {
				return nil, err
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	metrics, err := metrics.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	client := Client{
		Clientset: clientset,
		Metrics:   metrics,
	}

	return &client, nil
}

func buildInClusterConfig() kubernetes.Interface {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Can not get kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Can not create kubernetes client: %v", err)
	}

	return clientset
}

// GetSecret gets secret and base64 decoded
func (c *Client) GetSecret(k8sNamespace, secretName string) (map[string][]byte, error) {
	secretsClient := c.Clientset.CoreV1().Secrets(k8sNamespace)

	secrets, err := secretsClient.Get(context.TODO(), secretName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secrets.Data, nil
}

// CreateSecret creates secret
func (c *Client) CreateSecret(k8sNamespace, secretName string, data map[string][]byte) error {
	secretsClient := c.Clientset.CoreV1().Secrets(k8sNamespace)

	secretSpec := core_v1.Secret{
		ObjectMeta: meta_v1.ObjectMeta{Name: secretName},
		Data:       data,
	}

	// Create secret
	secret, err := secretsClient.Create(context.TODO(), &secretSpec, meta_v1.CreateOptions{})
	if err != nil {
		return err
	}

	if secret.ObjectMeta.Name != secretName {
		return fmt.Errorf("mismatched secret name %s", secret.ObjectMeta.Name)
	}
	return nil
}

// VerifySecret verifies the secret
func (c *Client) VerifySecret(k8sNamespace, secretName string) error {
	secretsClient := c.Clientset.CoreV1().Secrets(k8sNamespace)

	// Get a secret
	secret, err := secretsClient.Get(context.TODO(), secretName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}

	if secret.ObjectMeta.Name == secretName {
		return nil
	}
	return fmt.Errorf("unable to find secret %s under namespace %s", secretName, k8sNamespace)
}

func (c *Client) getDeployments(namespace, component string) (*v1.DeploymentList, error) {
	deploymentsClient := c.Clientset.AppsV1().Deployments(namespace)

	return deploymentsClient.List(context.TODO(), meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("component=%s", component),
	})
}

func (c *Client) getStatefulSets(namespace, component string) (*v1.StatefulSetList, error) {
	stsClient := c.Clientset.AppsV1().StatefulSets(namespace)

	return stsClient.List(context.TODO(), meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("component=%s", component),
	})
}
