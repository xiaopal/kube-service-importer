package controller

import (
	"context"

	ptypes "k8s.io/apimachinery/pkg/types"
)

func (c *endpointsImporter) notifyUpdate(key objectKey) {
	c.updateQueue.Add(key)
}

func (c *endpointsImporter) processUpdates(ctx context.Context) bool {
	item, quit := c.updateQueue.Get()
	if quit {
		return false
	}
	defer c.updateQueue.Done(item)
	target, targetOK := c.targets[item.(objectKey)]
	if !targetOK {
		return false
	}
	patch, patchOK, err := target.buildPatch()
	if err == nil {
		if patchOK {
			_, err = c.client.Resource(c.resource, target.key.namespace).Patch(target.key.name, ptypes.MergePatchType, patch)
			c.logger.Printf("%s/%s: updated", target.key.namespace, target.key.name)
		}
		if err == nil {
			c.updateQueue.Forget(item)
			return true
		}
	}
	c.logger.Printf("error processing %s/%s: (retries %d) %v", target.key.namespace, target.key.name, c.updateQueue.NumRequeues(item), err)
	c.updateQueue.AddRateLimited(item)
	return true
}
