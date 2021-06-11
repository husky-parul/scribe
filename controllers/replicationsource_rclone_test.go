// nolint
package controllers

import (
	// "context"
	// "time"

	"context"
	"fmt"

	// snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	// "github.com/operator-framework/operator-lib/status"
	//batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	scribev1alpha1 "github.com/backube/scribe/api/v1alpha1"
)

//nolint:dupl
var _ = Describe("ReplicationSource [rclone]", func() {
	fmt.Printf("Describing the replication source")
	var ctx = context.Background()
	var namespace *corev1.Namespace
	var rs *scribev1alpha1.ReplicationSource
	var srcPVC *corev1.PersistentVolumeClaim
	srcPVCCapacity := resource.MustParse("7Gi")
	//logger := zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter))
	//logger.

	// setup namespace && PVC
	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "scribe-rclone-test-",
			},
		}
		// crete ns
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		Expect(namespace.Name).NotTo(BeEmpty())

		srcPVC = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thesource",
				Namespace: namespace.Name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: srcPVCCapacity,
					},
				},
			},
		}

		fmt.Printf("Printing from within the test")
		rs = &scribev1alpha1.ReplicationSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "instance",
				Namespace: namespace.Name,
			},
			Spec: scribev1alpha1.ReplicationSourceSpec{
				SourcePVC: srcPVC.Name,
			},
		}
		RcloneContainerImage = DefaultRcloneContainerImage
		fmt.Printf("rs: %+v\n", rs)
	})
	AfterEach(func() {
		// delete each namespace on shutdown so resources can be reclaimed
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})
	JustBeforeEach(func() {
		// source pvc comes up
		Expect(k8sClient.Create(ctx, srcPVC)).To(Succeed())
		Expect(k8sClient.Create(ctx, rs)).To(Succeed())
		// wait for the ReplicationSource to actually come up
		Eventually(func() error {
			inst := &scribev1alpha1.ReplicationSource{}
			return k8sClient.Get(ctx, nameFor(rs), inst)
		}, maxWait, interval).Should(Succeed())
	})

	Context("when a schedule is not specified", func() {
		fmt.Printf("%+v\n", rs)
		var secret *v1.Secret
		BeforeEach(func() {
			// create a secret file for rclone
			secret = &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rclone-secret", // probbly wrong but w/e
					Namespace: rs.Namespace,
				},
				StringData: map[string]string{
					"rclone.conf": "foo", // from replicationsource_test.go:563
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfig: &secret.Name,
			}
		})
		// we should not be syncing again if no schedule is specified
		It("the next sync time is nil", func() {
			fmt.Printf("Enters the test for sync time nil")
			Consistently(func() bool {
				// replication source should exist within k8s cluster
				Expect(k8sClient.Get(ctx, nameFor(rs), rs)).To(Succeed())
				if rs.Status == nil || rs.Status.NextSyncTime.IsZero() {
					return false
				}
				return true
			}, duration, interval).Should(BeFalse())
			fmt.Printf("Exits the nil test")
		})
	})
})
