package controllers

import (
	"context"
	"fmt"
	scribev1alpha1 "github.com/backube/scribe/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ReplicationDestination", func() {
	var ctx = context.Background()
	var namespace *corev1.Namespace
	var rd *scribev1alpha1.ReplicationDestination
	var rcloneSecret *corev1.Secret

	var rcloneDestPath = "destpath"
	var rcloneConfigSection = "configsection"
	// dummy vars for rclone secret
	//var configSection = "hunter2"
	BeforeEach(func() {
		// new namespace for each test
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "scribe-rclone-test-",
			},
		}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		Expect(namespace.Name).NotTo(BeEmpty())

		// describe the ReplicationDestination
		rd = &scribev1alpha1.ReplicationDestination{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "instance",
				Namespace: namespace.Name,
			},
		}

		rcloneSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rclone-secret",
				Namespace: namespace.Name,
			},
			StringData: map[string]string{
				"rclone.conf": "hunter2",
			},
		}
		Expect(k8sClient.Create(ctx, rcloneSecret)).To(Succeed())
		RsyncContainerImage = DefaultRcloneContainerImage
	})
	AfterEach(func() {
		// cleanup the namespace after testing
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})
	JustBeforeEach(func() {
		Expect(k8sClient.Create(ctx, rd)).To(Succeed())
		// await ReplicationDestination to run
		Eventually(func() error {
			inst := &scribev1alpha1.ReplicationDestination{}
			return k8sClient.Get(ctx, nameFor(rd), inst)
		}, maxWait, interval).Should(Succeed())

		// make sure that our secret is actually getting insantiated
		Eventually(func() error {
			var secret = &corev1.Secret{}
			return k8sClient.Get(ctx, types.NamespacedName{Name: "rclone-secret", Namespace: rd.Namespace}, secret)
		}, maxWait, interval).Should(Succeed())
	})

	Context("Just trying to get it to exist hahaha", func() {
		BeforeEach(func() {
			fmt.Println("replicationdestination rclone object: %-v")
			rd.Spec.Rclone = &scribev1alpha1.ReplicationDestinationRcloneSpec{
				ReplicationDestinationVolumeOptions: scribev1alpha1.ReplicationDestinationVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfigSection: &rcloneConfigSection,
				RcloneDestPath:      &rcloneDestPath,
				RcloneConfig:        &rcloneSecret.Name,
			}
		})

		It("Basic test", func() {
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-dest-" + rd.Name, Namespace: rd.Namespace}, job)).To(Succeed())
		})
	})

	////nolint:dupl
	//Context("when a destinationPVC is specified", func() {
	//	var pvc *v1.PersistentVolumeClaim
	//	BeforeEach(func() {
	//		pvc = &v1.PersistentVolumeClaim{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Name:      "foo",
	//				Namespace: rd.Namespace,
	//			},
	//			Spec: v1.PersistentVolumeClaimSpec{
	//				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
	//				Resources: v1.ResourceRequirements{
	//					Requests: v1.ResourceList{
	//						"storage": resource.MustParse("10Gi"),
	//					},
	//				},
	//			},
	//		}
	//
	//		//accessModes := []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}
	//		// deploy the pvc
	//		Expect(k8sClient.Create(ctx, pvc)).To(Succeed())
	//		// create the rclone spec so it can *try* to create it
	//		//Expect(k8sClient.Get(
	//		//	ctx,
	//		//	types.NamespacedName{Name: rcloneSecret.Name, Namespace: rd.Namespace},
	//		//	&batchv1.Job{}),
	//		//).To(Succeed())
	//		rd.Spec.Rclone = &scribev1alpha1.ReplicationDestinationRcloneSpec{
	//			ReplicationDestinationVolumeOptions: scribev1alpha1.ReplicationDestinationVolumeOptions{
	//				DestinationPVC: &pvc.Name,
	//				CopyMethod:     scribev1alpha1.CopyMethodNone,
	//			},
	//			RcloneDestPath:      &rcloneDestPath,
	//			RcloneConfigSection: &rcloneConfigSection,
	//			RcloneConfig:        &rcloneSecret.Name,
	//		}
	//	})
	//	It("is used as the target PVC", func() {
	//		//job := &batchv1.Job{}
	//		//Eventually(func() error {
	//		//	return k8sClient.Get(ctx,
	//		//		types.NamespacedName{Name: "scribe-rclone-dest-" + rd.Name, Namespace: rd.Namespace}, job)
	//		//}, maxWait, interval).Should(Succeed())
	//		//volumes := job.Spec.Template.Spec.Volumes
	//		//found := false
	//		//for _, v := range volumes {
	//		//	if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvc.Name {
	//		//		found = true
	//		//	}
	//		//}
	//		//Expect(found).To(BeTrue())
	//		//Expect(pvc).NotTo(beOwnedBy(rd)) // we want this to be an independent PVC
	//
	//	})
	//})
	//
	//// nolint:dupl
	//Context("when capacity and accessModes are specified", func() {
	//	capacity := resource.MustParse("2Gi")
	//	accessModes := []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}
	//	BeforeEach(func() {
	//		rd.Spec.Rclone = &scribev1alpha1.ReplicationDestinationRcloneSpec{
	//			ReplicationDestinationVolumeOptions: scribev1alpha1.ReplicationDestinationVolumeOptions{
	//				Capacity:    &capacity,
	//				AccessModes: accessModes,
	//			},
	//			RcloneConfig:        &rcloneSecret.Name,
	//			RcloneDestPath:      &rcloneDestPath,
	//			RcloneConfigSection: &rcloneConfigSection,
	//		}
	//		// make sure rclone spec exists in ReplicationDestination
	//		//Expect(rd.Spec.Rclone).To(Not(BeEmpty()))
	//		fmt.Printf("%-v", rd.Spec.Rclone)
	//	})

	//nolint:dupl
	//It("creates a PVC", func() {
	//	fmt.Printf("%-v", rd.Spec.Rclone)
	//	job := &batchv1.Job{}
	//	Eventually(func() error {
	//		//nolint[:lll]
	//		return k8sClient.Get(ctx,
	//			types.NamespacedName{
	//				Name:      "scribe-rclone-dest-" + rd.Name,
	//				Namespace: rd.Namespace,
	//			}, job,
	//		)
	//	}, maxWait, interval).Should(Succeed())
	//	var pvcName string
	//	volumes := job.Spec.Template.Spec.Volumes
	//	for _, v := range volumes {
	//		if v.PersistentVolumeClaim != nil && v.Name == dataVolumeName {
	//			pvcName = v.PersistentVolumeClaim.ClaimName
	//		}
	//	}
	//	pvc := &v1.PersistentVolumeClaim{}
	//	Eventually(func() error {
	//		return k8sClient.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: rd.Namespace}, pvc)
	//	}, maxWait, interval).Should(Succeed())
	//	Expect(pvc).To(beOwnedBy(rd))
	//	Expect(*pvc.Spec.Resources.Requests.Storage()).To(Equal(capacity))
	//	Expect(pvc.Spec.AccessModes).To(Equal(accessModes))
	//})
})
