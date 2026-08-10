package main

import (
	"context"
	"fmt"
	"log"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"
)

// CreateInstance creates a new GCE instance with a container.
func (s *Server) CreateInstance(ctx context.Context, project, zone, instanceName, image, containerImage string) error {
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create instances client: %w", err)
	}
	defer instancesClient.Close()

	// Prepare the container declaration in YAML format
	containerDeclaration := fmt.Sprintf(`spec:
  hostNetwork: true
  containers:
  - name: %s
    image: %s
    stdin: true
    tty: true
  restartPolicy: Always
`, instanceName, containerImage)

	req := &computepb.InsertInstanceRequest{
		Project: project,
		Zone:    zone,
		InstanceResource: &computepb.Instance{
			Name:        proto.String(instanceName),
			MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/n1-standard-1", zone)),
			Disks: []*computepb.AttachedDisk{
				{
					AutoDelete: proto.Bool(true),
					Boot:       proto.Bool(true),
					Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						SourceImage: proto.String(image),
						DiskSizeGb:  proto.Int64(10),
					},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Name: proto.String("default"),
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name: proto.String("External NAT"),
							Type: proto.String(computepb.AccessConfig_ONE_TO_ONE_NAT.String()),
						},
					},
				},
			},
			Metadata: &computepb.Metadata{
				Items: []*computepb.Items{
					{
						Key:   proto.String("gce-container-declaration"),
						Value: proto.String(containerDeclaration),
					},
				},
			},
		},
	}

	op, err := instancesClient.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to insert instance: %w", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for operation: %w", err)
	}

	log.Printf("Instance %s created successfully", instanceName)
	return nil
}

// GetInstanceIP retrieves the external IP of a GCE instance.
func (s *Server) GetInstanceIP(ctx context.Context, project, zone, instanceName string) (string, error) {
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create instances client: %w", err)
	}
	defer instancesClient.Close()

	req := &computepb.GetInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instanceName,
	}

	instance, err := instancesClient.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}

	for _, ni := range instance.NetworkInterfaces {
		for _, ac := range ni.AccessConfigs {
			if ac.NatIP != nil {
				return *ac.NatIP, nil
			}
		}
	}

	return "", fmt.Errorf("no external IP found for instance %s", instanceName)
}

// GetInstanceInternalIP retrieves the internal IP of a GCE instance.
func (s *Server) GetInstanceInternalIP(ctx context.Context, project, zone, instanceName string) (string, error) {
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create instances client: %w", err)
	}
	defer instancesClient.Close()

	req := &computepb.GetInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instanceName,
	}

	instance, err := instancesClient.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}

	for _, ni := range instance.NetworkInterfaces {
		if ni.NetworkIP != nil {
			return *ni.NetworkIP, nil
		}
	}

	return "", fmt.Errorf("no internal IP found for instance %s", instanceName)
}

// DeleteInstance deletes a GCE instance.
func (s *Server) DeleteInstance(ctx context.Context, project, zone, instanceName string) error {
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create instances client: %w", err)
	}
	defer instancesClient.Close()

	req := &computepb.DeleteInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instanceName,
	}

	op, err := instancesClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for operation: %w", err)
	}

	log.Printf("Instance %s deleted successfully", instanceName)
	return nil
}

// ListInstances lists all GCE instances in a zone.
func (s *Server) ListInstances(ctx context.Context, project, zone string) ([]*computepb.Instance, error) {
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create instances client: %w", err)
	}
	defer instancesClient.Close()

	req := &computepb.ListInstancesRequest{
		Project: project,
		Zone:    zone,
	}

	var instances []*computepb.Instance
	it := instancesClient.List(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list instances: %w", err)
		}
		instances = append(instances, resp)
	}

	return instances, nil
}
