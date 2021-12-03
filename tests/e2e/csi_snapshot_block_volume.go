/*
Copyright 2019 The Kubernetes Authors.

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

package e2e

import (
	"context"
	//"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	//cnstypes "github.com/vmware/govmomi/cns/types"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	fnodes "k8s.io/kubernetes/test/e2e/framework/node"
	fpv "k8s.io/kubernetes/test/e2e/framework/pv"
	//"k8s.io/client-go/dynamic"
)

/*
	Tests to verify snapshots workflows for vanilla-block volumes
*/

var _ = ginkgo.Describe("[csi-block-vanilla] Volume Snapshot Test ", func() {
	f := framework.NewDefaultFramework("volume-snapshot")
	var (
		client            clientset.Interface
		namespace         string
		scParameters      map[string]string
		datastoreURL      string
		pvclaims          []*v1.PersistentVolumeClaim
		// storagePolicyName string
	)
	ginkgo.BeforeEach(func() {
		bootstrap()
		client = f.ClientSet
		namespace = getNamespaceToRunTests(f)
		scParameters = make(map[string]string)
		datastoreURL = GetAndExpectStringEnvVar(envSharedDatastoreURL)
		// storagePolicyName = GetAndExpectStringEnvVar(envStoragePolicyNameForSharedDatastores)
		nodeList, err := fnodes.GetReadySchedulableNodes(f.ClientSet)
		framework.ExpectNoError(err, "Unable to find ready and schedulable Node")
		if !(len(nodeList.Items) > 0) {
			framework.Failf("Unable to find ready and schedulable Node")
		}

	})

	ginkgo.AfterEach(func() {
		if supervisorCluster {
			deleteResourceQuota(client, namespace)
		}
		if guestCluster {
			svcClient, svNamespace := getSvcClientAndNamespace()
			setResourceQuota(svcClient, svNamespace, defaultrqLimit)
		}

	})


	/*
	Create/Delete snapshot via k8s API using PVC as source (Dynamic Provisioning)

	Test Steps

	1. Create a storage class (eg: vsan-default) and create a pvc using this sc
	2. Wait for PV creation and PVC to be in Bound state
	3. Create a VolumeSnapshot class with driver as vsphere-csi-driver and deletionPolicy set to Delete
	4. Create a volume-snapshot using the above snapshot-class and pvc (from step-1) as source
	5. Ensure the snapshot is created, verify using get VolumeSnapshot
	6. Verify that VolumeSnapshotContent is auto-created 
	7. Delete the above snapshot, run a get and ensure its removed
	8. Since the snapshotclass used has Delete policy, ensure the VolumeSnapshotContent is deleted 
	
	*/


	ginkgo.It("Verify snapshot dynamic provisioning workflow", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ginkgo.By("Invoking Test for snapshot")
		var storageclass *storagev1.StorageClass
		var pvclaim *v1.PersistentVolumeClaim
		var err error
		// decide which test setup is available to run
		if vanillaCluster {
			ginkgo.By("CNS_TEST: Running for vanilla k8s setup")
			scParameters[scParamDatastoreURL] = datastoreURL
			storageclass, pvclaim, err = createPVCAndStorageClass(client,
				namespace, nil, scParameters, diskSize, nil, "", false, "")
		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		defer func() {
			err := client.StorageV1().StorageClasses().Delete(ctx, storageclass.Name, *metav1.NewDeleteOptions(0))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}()

		ginkgo.By("Expect claim to provision volume successfully")
		err = fpv.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, client,
			pvclaim.Namespace, pvclaim.Name, framework.Poll, time.Minute)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to provision volume")

		pvclaims = append(pvclaims, pvclaim)

		persistentvolumes, err := fpv.WaitForPVClaimBoundPhase(client, pvclaims, framework.ClaimProvisionTimeout)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		volHandle := persistentvolumes[0].Spec.CSI.VolumeHandle
		gomega.Expect(volHandle).NotTo(gomega.BeEmpty())

		defer func() {
			err := fpv.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = e2eVSphere.waitForCNSVolumeToBeDeleted(volHandle)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}()

		// WaitForSnapshotReady(dynamic.Interface, namespace, pvclaim.Name, 20, 300)
		//snapshot := getSnapshot(client, namespace)
		// createDeleteSnapshot(pvclaim.Name, namespace, "example-vanilla-block-snapclass", "test-snapshot-")
		createVolumeSnapshotClass("test-snap-class")
		/*
		ginkgo.By(fmt.Sprintf("Invoking QueryCNSVolumeWithResult with VolumeID: %s", volHandle))
		queryResult, err := e2eVSphere.queryCNSVolumeWithResult(volHandle)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if len(queryResult.Volumes) == 0 {
			err = fmt.Errorf("QueryCNSVolumeWithResult returned no volume")
		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		ginkgo.By("Verifying disk size specified in PVC in honored")
		if queryResult.Volumes[0].BackingObjectDetails.(*cnstypes.CnsBlockBackingDetails).CapacityInMb != diskSizeInMb {
			err = fmt.Errorf("wrong disk size provisioned ")
		}
		*/
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})
})
