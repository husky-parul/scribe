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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	scribev1alpha1 "github.com/backube/scribe/api/v1alpha1"
)

//nolint:dupl
var _ = Describe("ReplicationDestination [rclone]", func() {

	var ctx = context.Background()
	var namespace *corev1.Namespace
	var rd *scribev1alpha1.ReplicationDestination
	var rcloneSecret *corev1.Secret

	var configSection = "foo"
	var destPath = "bar"
	var pvc *corev1.PersistentVolumeClaim

	// setup namespace && PVC

	// ***************************** this fails

	//nolint:dupl
	Context("when a destinationPVC is specified", func() {

		BeforeEach(func() {
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "scribe-rclone-src-",
				},
			}
			// crete ns
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
			Expect(namespace.Name).NotTo(BeEmpty())

			// need secret for most of these tests to work
			rcloneSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rclone-secret",
					Namespace: namespace.Name,
				},
				StringData: map[string]string{
					"rclone.conf": "hunter2",
				},
			}

			rd = &scribev1alpha1.ReplicationDestination{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance",
					Namespace: namespace.Name,
				},
			}
			rd.Spec.Rclone = &scribev1alpha1.ReplicationDestinationRcloneSpec{
				ReplicationDestinationVolumeOptions: scribev1alpha1.ReplicationDestinationVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfigSection: &configSection,
				RcloneDestPath:      &destPath,
				RcloneConfig:        &rcloneSecret.Name,
			}
			RcloneContainerImage = DefaultRcloneContainerImage

			pvc = &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: rd.Namespace,
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"storage": resource.MustParse("2Gi"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())
			rd.Spec.Rclone = &scribev1alpha1.ReplicationDestinationRcloneSpec{
				ReplicationDestinationVolumeOptions: scribev1alpha1.ReplicationDestinationVolumeOptions{
					DestinationPVC: &pvc.Name,
				},
			}
		})
		AfterEach(func() {
			// delete each namespace on shutdown so resources can be reclaimed
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})
		JustBeforeEach(func() {
			// source pvc comes up
			Expect(k8sClient.Create(ctx, rcloneSecret)).To(Succeed())
			Expect(k8sClient.Create(ctx, rd)).To(Succeed())
			// wait for the ReplicationDestination to actually come up
			Eventually(func() error {
				inst := &scribev1alpha1.ReplicationDestination{}
				return k8sClient.Get(ctx, nameFor(rd), inst)
			}, maxWait, interval).Should(Succeed())
		})
		// --------

		It("Test if job finishes", func() {

			rd.Spec.Rclone = &scribev1alpha1.ReplicationDestinationRcloneSpec{
				ReplicationDestinationVolumeOptions: scribev1alpha1.ReplicationDestinationVolumeOptions{
					DestinationPVC: &pvc.Name,
				},
			}

			Eventually(func() error {
				job := &batchv1.Job{}
				found := k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rd.Name, Namespace: rd.Namespace}, job)
				// inst := &scribev1alpha1.ReplicationDestination{}
				// rdName := k8sClient.Get(ctx, nameFor(rd), inst)

				fmt.Printf("rd is +++++++++++++++++++------------------------------------: %-v \n", rd.Name)
				fmt.Printf("rd is +++++++++++++++++++------------------------------------: %-v \n", rd.Namespace)

				// fmt.Printf("rdName is +++++++++++++++++++ \n ------------------------------------: %-v", rdName)

				fmt.Printf("Found is +++++++++++++++++++------------------------------------: %-v\n", found)

				return found
			}, maxWait, interval).Should(Succeed())
			Expect(true).To(BeTrue())
			Expect(pvc).NotTo(beOwnedBy(rd))

		})
	})

})
