// nolint
package controllers

import (
	// "context"
	// "time"

	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"

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
	var ctx = context.Background()
	var namespace *corev1.Namespace
	var rs *scribev1alpha1.ReplicationSource
	var srcPVC *corev1.PersistentVolumeClaim
	var rcloneSecret *corev1.Secret
	srcPVCCapacity := resource.MustParse("7Gi")

	// dummy variables taken from https://scribe-replication.readthedocs.io/en/latest/usage/rclone/index.html#source-configuration
	var configSection = "foo"
	var destPath = "bar"
	//var config = "foobar"
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
	})
	AfterEach(func() {
		// delete each namespace on shutdown so resources can be reclaimed
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})
	JustBeforeEach(func() {
		// source pvc comes up
		Expect(k8sClient.Create(ctx, srcPVC)).To(Succeed())
		Expect(k8sClient.Create(ctx, rcloneSecret)).To(Succeed())
		Expect(k8sClient.Create(ctx, rs)).To(Succeed())
		// wait for the ReplicationSource to actually come up
		Eventually(func() error {
			inst := &scribev1alpha1.ReplicationSource{}
			return k8sClient.Get(ctx, nameFor(rs), inst)
		}, maxWait, interval).Should(Succeed())
	})

	Context("when a schedule is not specified", func() {
		fmt.Printf("Replication Source %+v\n", rs)
		BeforeEach(func() {
			// changing this block from Rsync to Rclone causes the unit
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfigSection: &configSection,
				RcloneDestPath:      &destPath,
				RcloneConfig:        &rcloneSecret.Name,
			}
		})
		// we should not be syncing again if no schedule is specified
		It("the next sync time is nil", func() {
			Consistently(func() bool {
				// replication source should exist within k8s cluster
				Expect(k8sClient.Get(ctx, nameFor(rs), rs)).To(Succeed())
				if rs.Status == nil || rs.Status.NextSyncTime.IsZero() {
					return false
				}
				return true
			}, duration, interval).Should(BeFalse())
		})
	})

	Context("When a copyMethod of None is specified for Rclone", func() {
		BeforeEach(func() {
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfigSection: &configSection,
				RcloneDestPath:      &destPath,
				RcloneConfig:        &rcloneSecret.Name,
			}
		})

		It("Uses the Source PVC as the sync source", func() {
			job := &batchv1.Job{}
			Eventually(func() error {
				fmt.Printf("ReplicationSource so far: %+v\n", rs)
				return k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rs.Name, Namespace: rs.Namespace}, job)
			}, maxWait, interval).Should(Succeed())
			volumes := job.Spec.Template.Spec.Volumes
			found := false
			for _, v := range volumes {
				if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == srcPVC.Name {
					found = true
				}
			}
			Expect(found).To(BeTrue())
			Expect(srcPVC).NotTo(beOwnedBy(rs))
		})
	})

	Context("when a copyMethod of Clone is specified", func() {
		BeforeEach(func() {
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodClone,
				},
				RcloneConfigSection: &configSection,
				RcloneDestPath:      &destPath,
				RcloneConfig:        &rcloneSecret.Name,
			}
		})

		// attempts to invoke rloneReconcile
		//nolint:dupl
		It("creates a clone of the source PVC as the sync source", func() {
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rs.Name, Namespace: rs.Namespace}, job)
			}, maxWait, interval).Should(Succeed())
			volumes := job.Spec.Template.Spec.Volumes
			pvc := &corev1.PersistentVolumeClaim{}
			pvc.Namespace = rs.Namespace
			found := false
			for _, v := range volumes {
				if v.PersistentVolumeClaim != nil {
					found = true
					pvc.Name = v.PersistentVolumeClaim.ClaimName
				}
			}
			Expect(found).To(BeTrue())
			Expect(k8sClient.Get(ctx, nameFor(pvc), pvc)).To(Succeed())
			Expect(pvc.Spec.DataSource.Name).To(Equal(srcPVC.Name))
			Expect(pvc).To(beOwnedBy(rs))

		})
	})

	// todo: test for sync job being paused
	// taken after this: https://github.com/backube/scribe/blob/e81281ac0fc1d50c3ae31369495f3c5fe3927a8f/controllers/replicationsource_test.go#L426
	Context("Pausing an rclone sync job", func() {
		parallelism := int32(0)
		BeforeEach(func() {
			rs.Spec.Paused = true
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodClone,
				},
				RcloneConfigSection: &configSection,
				RcloneDestPath:      &destPath,
				RcloneConfig:        &rcloneSecret.Name,
			}
		})
		It("job will be created but won't start", func() {
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rs.Name, Namespace: rs.Namespace}, job)
			}, maxWait, interval).Should(Succeed())
			Expect(*job.Spec.Parallelism).To(Equal(parallelism))
		})
	})

	// todo: test rcloneSrcReconciler.cleanupJob
	//

	// todo: test invalid rclone Spec
	// we want to target a job
	Context("When rclone fails to validate the spec", func() {
		var emptyString = ""
		BeforeEach(func() {
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfig:        &emptyString,
				RcloneConfigSection: &emptyString,
				RcloneDestPath:      &emptyString,
			}
		})
		It("No spec at all", func() {
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rs.Name, Namespace: rs.Namespace}, job)).NotTo(Succeed())
		})

	})

	Context("rclone has secret but nothing else", func() {
		BeforeEach(func() {
			var emptyString = ""
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfig:        &rcloneSecret.Name,
				RcloneConfigSection: &emptyString,
				RcloneDestPath:      &emptyString,
			}
		})
		It("running rclone with a broken config", func() {
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rs.Name, Namespace: rs.Namespace}, job)).NotTo(Succeed())
		})
	})

	Context("rclone has secret + rcloneConfigSection but not DestPath", func() {
		BeforeEach(func() {
			var emptyString = ""
			rs.Spec.Rclone = &scribev1alpha1.ReplicationSourceRcloneSpec{
				ReplicationSourceVolumeOptions: scribev1alpha1.ReplicationSourceVolumeOptions{
					CopyMethod: scribev1alpha1.CopyMethodNone,
				},
				RcloneConfig:        &rcloneSecret.Name,
				RcloneConfigSection: &configSection,
				RcloneDestPath:      &emptyString,
			}
		})
		It("Existing RcloneConfig + RcloneConfigSection", func() {
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "scribe-rclone-src-" + rs.Name, Namespace: rs.Namespace}, job)).NotTo(Succeed())
		})
	})

	//Context("When the rclone secret isn't specified", func() {
	//	BeforeEach(func() {
	//		var rcloneSecret = "foobar"
	//		rs.Spec.Rclone = {
	//
	//		}
	//	})
	//})
})
