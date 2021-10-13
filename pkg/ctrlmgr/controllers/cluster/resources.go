/*
Copyright 2021 KubeCube Authors

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

package controllers

import (
	"context"
	"fmt"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
)

const (
	configMapName = "kubeconfig-pivot-cluster"
	secretName    = "cube-tls-secret"
	webhookName   = "warden-validating-webhook-configuration"
	appKey        = "kubecube.io/app"
)

func deployResources(ctx context.Context, memberCluster, pivotCluster clusterv1.Cluster) error {
	// get target memberCluster client
	cli := clients.Interface().Kubernetes(memberCluster.Name)
	if cli == nil {
		return fmt.Errorf("get internal client failed")
	}

	c := cli.Direct()

	isMemberCluster := memberCluster.Spec.IsMemberCluster

	// create resource below when cluster is member
	if isMemberCluster {
		// create namespace to member cluster
		ns := makeNamespace()
		err := createResource(ctx, ns, c, memberCluster.Name, "namespace")
		if err != nil {
			return err
		}

		// create tlsSecret to member cluster
		secret := makeTLSSecret()
		err = createResource(ctx, secret, c, memberCluster.Name, "secret")
		if err != nil {
			return err
		}

		// create crds to member cluster
		crds := makeCRDs()
		for _, crd := range crds {
			err := createResource(ctx, crd, c, memberCluster.Name, "crd")
			if err != nil {
				return err
			}
		}

		// create kubeConfig cm to member cluster
		cm := makeKubeConfigCM(pivotCluster)
		err = createResource(ctx, cm, c, memberCluster.Name, "configmap")
		if err != nil {
			return err
		}

		clusterRole := makeClusterRole()
		err = createResource(ctx, clusterRole, c, memberCluster.Name, "ClusterRole")
		if err != nil {
			return err
		}

		clusterRoleBinding := makeClusterRoleBinding()
		err = createResource(ctx, clusterRoleBinding, c, memberCluster.Name, "ClusterRoleBinding")
		if err != nil {
			return err
		}
	}

	// create warden deployment to member cluster
	deployment := makeDeployment(memberCluster.Name, isMemberCluster)
	err := createResource(ctx, deployment, c, memberCluster.Name, "deployment")
	if err != nil {
		return err
	}

	// create service to warden
	svc := makeClusterIpSvc()
	err = createResource(ctx, svc, c, memberCluster.Name, "service")
	if err != nil {
		return err
	}

	npSvc := makeNodePortSvc()
	err = createResource(ctx, npSvc, c, memberCluster.Name, "service")
	if err != nil {
		clog.Warn("NodePort service %v already exist: %v", npSvc.Name, err)
	}

	// create validate webhook to member cluster
	wh := makeWardenWebhook()
	err = createResource(ctx, wh, c, memberCluster.Name, "validateWebhookConfiguration")
	if err != nil {
		return err
	}

	return nil
}

// makeDeployment set kubeconfig and jwt secret for warden
func makeDeployment(cluster string, isMemberCluster bool) *appsv1.Deployment {
	var (
		label = map[string]string{appKey: constants.Warden}

		args = []string{
			"-pivot-cluster-kubeconfig=/etc/config/kubeconfig",
			"-tls-cert=/etc/tls/tls.crt",
			"-tls-key=/etc/tls/tls.key",
			fmt.Sprintf("-cluster=%s", cluster),
			fmt.Sprintf("-pivot-cube-host=%s", env.PivotCubeHost()),
		}

		tlsVolume = corev1.Volume{
			Name: secretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}

		helmVolume = corev1.Volume{
			Name: "helm-pkg",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}

		timeZoneVolume = corev1.Volume{
			Name: "localtime",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/localtime",
				},
			},
		}

		configVolume = corev1.Volume{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "config",
							Path: "kubeconfig",
						},
					},
				},
			},
		}

		tlsVolumeMount      = corev1.VolumeMount{Name: secretName, MountPath: "/etc/tls", ReadOnly: true}
		helmVolumeMount     = corev1.VolumeMount{Name: "helm-pkg", MountPath: "/root/helmchartpkg"}
		timeZoneVolumeMount = corev1.VolumeMount{Name: "localtime", MountPath: "/etc/localtime"}
		configVolumeMount   = corev1.VolumeMount{Name: "config-volume", MountPath: "/etc/config", ReadOnly: true}

		volumeMounts = []corev1.VolumeMount{
			configVolumeMount,
			timeZoneVolumeMount,
			helmVolumeMount,
			tlsVolumeMount,
		}

		volumes = []corev1.Volume{
			configVolume,
			helmVolume,
			tlsVolume,
			timeZoneVolume,
		}
	)

	if !isMemberCluster {
		volumeMounts = []corev1.VolumeMount{tlsVolumeMount, helmVolumeMount, timeZoneVolumeMount}
		volumes = []corev1.Volume{tlsVolume, helmVolume, timeZoneVolume}
		args = []string{
			"-in-member-cluster=false",
			"-tls-cert=/etc/tls/tls.crt",
			"-tls-key=/etc/tls/tls.key",
			fmt.Sprintf("-cluster=%s", constants.PivotCluster),
			fmt.Sprintf("-pivot-cube-host=%s", env.PivotCubeClusterIPSvc()),
		}
	}

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.Warden,
			Namespace: constants.CubeNamespace,
			Labels:    label,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "downloader",
							Image: env.WardenInitImage(),
							Env: []corev1.EnvVar{
								{Name: "DOWNLOAD_CHARTS", Value: env.ChartsDownload()},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "helm-pkg",
									MountPath: "/root/helmchartpkg",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  constants.Warden,
							Image: env.WardenImage(),
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  int64Ptr(0),
								Privileged: boolPtr(true),
							},
							Args: args,
							Env: []corev1.EnvVar{
								{Name: "JWT_SECRET", Value: env.JwtSecret()},
								{Name: "GIN_MODE", Value: "release"},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	return d
}

// makeKubeConfigCM make configmap container kubeConfig of pivot cluster
func makeKubeConfigCM(pivotCluster clusterv1.Cluster) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: constants.CubeNamespace,
		},
		Data: map[string]string{"config": string(pivotCluster.Spec.KubeConfig)},
	}

	return cm
}

// makeClusterIpSvc make service to be used for auth
func makeClusterIpSvc() *corev1.Service {
	label := map[string]string{
		appKey: constants.Warden,
	}

	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.Warden,
			Namespace: constants.CubeNamespace,
			Labels:    label,
		},
		Spec: corev1.ServiceSpec{
			Selector: label,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: 7443,
				},
				{
					Name: "webhook",
					Port: 8443,
				},
			},
		},
	}

	return s
}

func makeNodePortSvc() *corev1.Service {
	label := map[string]string{
		appKey: constants.Warden,
	}

	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "warden-nodeport",
			Namespace: constants.CubeNamespace,
			Labels:    label,
		},
		Spec: corev1.ServiceSpec{
			Selector: label,
			Type:     corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       7443,
					TargetPort: intstr.FromInt(7443),
					NodePort:   31443,
				},
			},
		},
	}

	return s
}

func makeNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.CubeNamespace,
		},
	}
}

func makeCRDs() (crds []*apiextensionsv1.CustomResourceDefinition) {
	pClient := clients.Interface().Kubernetes(constants.PivotCluster).Cache()
	crdList := apiextensionsv1.CustomResourceDefinitionList{}

	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.CrdLabel, true))
	if err != nil {
		log.Error(err.Error())
	}

	err = pClient.List(context.Background(), &crdList, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Error(err.Error())
	}

	for _, crd := range crdList.Items {
		crds = append(crds, &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:   crd.Name,
				Labels: crd.Labels,
			},
			Spec:   crd.Spec,
			Status: crd.Status,
		})
	}

	return
}

func makeWardenWebhook() *v1.ValidatingWebhookConfiguration {
	pClient := clients.Interface().Kubernetes(constants.PivotCluster).Cache()
	wh := v1.ValidatingWebhookConfiguration{}
	key := types.NamespacedName{Name: webhookName}
	err := pClient.Get(context.Background(), key, &wh)
	if err != nil {
		log.Error(err.Error())
	}

	return &v1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: wh.Name,
		},
		Webhooks: wh.Webhooks,
	}
}

func makeTLSSecret() *corev1.Secret {
	pClient := clients.Interface().Kubernetes(constants.PivotCluster).Cache()
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      secretName,
		Namespace: constants.CubeNamespace,
	}
	err := pClient.Get(context.Background(), key, &secret)
	if err != nil {
		log.Error(err.Error())
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Type: secret.Type,
		Data: secret.Data,
	}
}

func makeClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubecube-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
		},
	}
}

func makeClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubecube-rolebinding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: constants.K8sGroupRBAC,
			Kind:     constants.K8sKindClusterRole,
			Name:     "kubecube-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      "default",
				Kind:      constants.K8sKindServiceAccount,
				Namespace: constants.CubeNamespace,
			},
		},
	}
}
