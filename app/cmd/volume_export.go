package cmd

import (
	"fmt"
	"log"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/longhorn/longhorn-engine/pkg/replica"
	replicaclient "github.com/longhorn/longhorn-engine/pkg/replica/client"
	"github.com/longhorn/longhorn-engine/pkg/types"
)

func ExportVolumeCmd() cli.Command {
	return cli.Command{
		Name:      "export-volume",
		ShortName: "export",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "snapshot-name",
				Usage: "specify the name of the volume's snapshot to export",
			},
			cli.StringFlag{
				Name:  "receiver-address",
				Usage: "specify the address of the receiver",
			},
			cli.IntFlag{
				Name:  "receiver-port",
				Usage: "specify the port of the receiver",
			},
			cli.BoolFlag{
				Name:  "export-backing-image-if-exist",
				Usage: "specify if the backing image should be exported if it exists",
			},
		},
		Action: func(c *cli.Context) {
			if err := exportVolume(c); err != nil {
				log.Fatalf("Effor running export volume command: %v", err)
			}
		},
	}
}

func exportVolume(c *cli.Context) error {
	// Validate arguments
	snapshotName := c.String("snapshot-name")
	if snapshotName == "" {
		return fmt.Errorf("missing required parameter: snapshot-name")
	}
	receiverAddress := c.String("receiver-address")
	if receiverAddress == "" {
		return fmt.Errorf("missing required parameter: receiver-address")
	}
	receiverPort := c.Int("receiver-port")
	if receiverPort == 0 {
		return fmt.Errorf("missing required parameter: receiver-port")
	}
	exportBackingImageIfExist := c.Bool("export-backing-image-if-exist")

	// Get controller url
	controllerClient, err := getControllerClient(c)
	if err != nil {
		return err
	}
	defer controllerClient.Close()
	volume, err := controllerClient.VolumeGet()
	if err != nil {
		return err
	}

	// Find a healthy replica
	var r *types.ControllerReplicaInfo
	replicas, err := controllerClient.ReplicaList()
	if err != nil {
		return err
	}
	for _, rep := range replicas {
		if rep.Mode == types.RW {
			r = rep
			break
		}
	}
	if r == nil {
		return fmt.Errorf("cannot find a RW replica for volume exporting")
	}

	rClient, err := replicaclient.NewReplicaClient(r.Address)
	if err != nil {
		return err
	}
	defer rClient.Close()

	rInfo, err := rClient.GetReplica()
	if err != nil {
		return err
	}

	// Check the snapshot in the replica
	diskName := replica.GenerateSnapshotDiskName(snapshotName)
	if _, ok := rInfo.Disks[diskName]; !ok {
		return fmt.Errorf("snapshot disk %s not found on replica %s", diskName, r.Address)
	}

	logrus.Infof("Exporting snapshot %v of volume %s using replica %v", snapshotName, volume.Name, r.Address)

	return rClient.ExportVolume(snapshotName, receiverAddress, int32(receiverPort), exportBackingImageIfExist)
}
