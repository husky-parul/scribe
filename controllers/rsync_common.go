/*
Copyright 2020 The Scribe authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type rsyncSvcDescription struct {
	Context  context.Context
	Client   client.Client
	Scheme   *runtime.Scheme
	Service  *corev1.Service
	Owner    metav1.Object
	Type     *corev1.ServiceType
	Selector map[string]string
	Port     *int32
}

func (d *rsyncSvcDescription) Reconcile(l logr.Logger) (bool, error) {
	logger := l.WithValues("service", nameFor(d.Service))

	op, err := ctrlutil.CreateOrUpdate(d.Context, d.Client, d.Service, func() error {
		if err := ctrl.SetControllerReference(d.Owner, d.Service, d.Scheme); err != nil {
			logger.Error(err, "unable to set controller reference")
			return err
		}
		d.Service.ObjectMeta.Annotations = map[string]string{
			"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
		}
		if d.Type != nil {
			d.Service.Spec.Type = *d.Type
		} else {
			d.Service.Spec.Type = corev1.ServiceTypeClusterIP
		}
		d.Service.Spec.Selector = d.Selector
		if len(d.Service.Spec.Ports) != 1 {
			d.Service.Spec.Ports = []corev1.ServicePort{{}}
		}
		d.Service.Spec.Ports[0].Name = "ssh"
		if d.Port != nil {
			d.Service.Spec.Ports[0].Port = *d.Port
		} else {
			d.Service.Spec.Ports[0].Port = 22
		}
		d.Service.Spec.Ports[0].Protocol = corev1.ProtocolTCP
		d.Service.Spec.Ports[0].TargetPort = intstr.FromInt(22)
		if d.Service.Spec.Type == corev1.ServiceTypeClusterIP {
			d.Service.Spec.Ports[0].NodePort = 0
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "Service reconcile failed")
		return false, err
	}

	logger.V(1).Info("Service reconciled", "operation", op)
	return true, nil
}

func getServiceAddress(svc *corev1.Service) string {
	address := svc.Spec.ClusterIP
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			if svc.Status.LoadBalancer.Ingress[0].Hostname != "" {
				address = svc.Status.LoadBalancer.Ingress[0].Hostname
			} else if svc.Status.LoadBalancer.Ingress[0].IP != "" {
				address = svc.Status.LoadBalancer.Ingress[0].IP
			}
		} else {
			address = ""
		}
	}
	return address
}

func getAndValidateSecret(ctx context.Context, client client.Client, logger logr.Logger,
	secret *corev1.Secret, fields []string) error {
	if err := client.Get(ctx, nameFor(secret), secret); err != nil {
		logger.Error(err, "failed to get Secret with provided name", "Secret", nameFor(secret))
		return err
	}
	if !secretHasFields(secret, fields) {
		err := fmt.Errorf("validation failed")
		logger.Error(err, "SSH keys Secret does not contain the proper fields", "Secret", nameFor(secret))
		return err
	}
	return nil
}

func secretHasFields(secret *corev1.Secret, fields []string) bool {
	data := secret.Data
	if data == nil || len(data) != len(fields) {
		return false
	}
	for _, k := range fields {
		if _, found := data[k]; !found {
			return false
		}
	}
	return true
}

type rsyncSSHKeys struct {
	Context      context.Context
	Client       client.Client
	Scheme       *runtime.Scheme
	Owner        metav1.Object
	NameTemplate string
	MainSecret   *corev1.Secret
	SrcSecret    *corev1.Secret
	DestSecret   *corev1.Secret
}

func (k *rsyncSSHKeys) Reconcile(l logr.Logger) (bool, error) {
	k.MainSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.NameTemplate + "-main-" + k.Owner.GetName(),
			Namespace: k.Owner.GetNamespace(),
		},
	}
	k.SrcSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.NameTemplate + "-src-" + k.Owner.GetName(),
			Namespace: k.Owner.GetNamespace(),
		},
	}
	k.DestSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.NameTemplate + "-dest-" + k.Owner.GetName(),
			Namespace: k.Owner.GetNamespace(),
		},
	}
	return reconcileBatch(l,
		k.ensureMainSecret,
		k.ensureSrcSecret,
		k.ensureDestSecret,
	)
}

func (k *rsyncSSHKeys) ensureMainSecret(l logr.Logger) (bool, error) {
	// The secrets hold the ssh key pairs to ensure mutual authentication of the
	// connection. The main secret holds both keys and is used ensure the source
	// & destination secrets remain consistent with each other.
	//
	// Since the key generation creates unique keys each time it's run, we can't
	// do much to reconcile the main secret. All we can do is:
	// - Create it if it doesn't exist
	// - Ensure the expected fields are present within
	logger := l.WithValues("mainSecret", nameFor(k.MainSecret))

	// See if it exists and has the proper fields
	err := k.Client.Get(k.Context, nameFor(k.MainSecret), k.MainSecret)
	if err != nil && !kerrors.IsNotFound(err) {
		logger.Error(err, "failed to get secret")
		return false, err
	}
	if err == nil { // found it, make sure it has the right fields
		if !secretHasFields(k.MainSecret, []string{"source", "source.pub", "destination", "destination.pub"}) {
			logger.V(1).Info("deleting invalid secret")
			if err = k.Client.Delete(k.Context, k.MainSecret); err != nil {
				logger.Error(err, "failed to delete secret")
			}
			return false, err
		}
		// Secret is valid, we're done
		logger.V(1).Info("secret is valid")
		return true, nil
	}

	// Need to create the secret
	if err = k.generateMainSecret(l); err != nil {
		l.Error(err, "unable to generate main secret")
		return false, err
	}
	if err = k.Client.Create(k.Context, k.MainSecret); err != nil {
		l.Error(err, "unable to create secret")
		return false, err
	}

	l.V(1).Info("created secret")
	return false, nil
}

func generateKeyPair(ctx context.Context) (private []byte, public []byte, err error) {
	keydir, err := ioutil.TempDir("", "sshkeys")
	if err != nil {
		return
	}
	defer os.RemoveAll(keydir)
	filename := filepath.Join(keydir, "key")
	if err = exec.CommandContext(ctx, "ssh-keygen", "-q", "-t", "rsa", "-b", "4096",
		"-f", filename, "-C", "", "-N", "").Run(); err != nil {
		return
	}
	if private, err = ioutil.ReadFile(filename); err != nil {
		return
	}
	public, err = ioutil.ReadFile(filename + ".pub")
	return
}

func (k *rsyncSSHKeys) generateMainSecret(l logr.Logger) error {
	k.MainSecret.Data = make(map[string][]byte, 4)
	if err := ctrl.SetControllerReference(k.Owner, k.MainSecret, k.Scheme); err != nil {
		l.Error(err, "unable to set controller reference")
		return err
	}

	priv, pub, err := generateKeyPair(k.Context)
	if err != nil {
		l.Error(err, "unable to generate source ssh keys")
		return err
	}
	k.MainSecret.Data["source"] = priv
	k.MainSecret.Data["source.pub"] = pub

	priv, pub, err = generateKeyPair(k.Context)
	if err != nil {
		l.Error(err, "unable to generate destination ssh keys")
		return err
	}
	k.MainSecret.Data["destination"] = priv
	k.MainSecret.Data["destination.pub"] = pub

	l.V(1).Info("created secret")
	return nil
}

func (k *rsyncSSHKeys) ensureSecret(l logr.Logger, secret *corev1.Secret, keys []string) (bool, error) {
	logger := l.WithValues("secret", nameFor(secret))

	op, err := ctrlutil.CreateOrUpdate(k.Context, k.Client, secret, func() error {
		if err := ctrl.SetControllerReference(k.Owner, secret, k.Scheme); err != nil {
			logger.Error(err, "unable to set controller reference")
			return err
		}
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, 3)
		}
		for _, key := range keys {
			secret.Data[key] = k.MainSecret.Data[key]
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "reconcile failed")
	} else {
		logger.V(1).Info("reconciled", "operation", op)
	}
	return true, err
}

func (k *rsyncSSHKeys) ensureSrcSecret(l logr.Logger) (bool, error) {
	logger := l.WithValues("sourceSecret", nameFor(k.SrcSecret))
	return k.ensureSecret(logger, k.SrcSecret, []string{"source", "source.pub", "destination.pub"})
}

func (k *rsyncSSHKeys) ensureDestSecret(l logr.Logger) (bool, error) {
	logger := l.WithValues("destSecret", nameFor(k.DestSecret))
	return k.ensureSecret(logger, k.DestSecret, []string{"destination", "destination.pub", "source.pub"})
}
