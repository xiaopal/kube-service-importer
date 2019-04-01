package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"

	src "github.com/xiaopal/kube-service-importer/pkg/controller/source"
	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
	"github.com/xiaopal/kube-service-importer/pkg/prober"
	corev1 "k8s.io/api/core/v1"
)

type objectKey struct {
	namespace string
	name      string
}

type hostKey struct {
	ip   string
	port int32
}

type probeKey struct {
	objectKey
	hostKey
	probe string
}

type sourceKey struct {
	objectKey
	source string
}

type targetRecord struct {
	c                       *endpointsImporter
	key                     objectKey
	subsets                 atomic.Value
	probeConfs, sourceConfs []fluconf.Config
	probes                  map[probeKey]prober.StatusProber
	sources                 map[sourceKey]prober.StatusProber
}

func (h *targetRecord) lastSubsets() []corev1.EndpointSubset {
	subsets := h.subsets.Load().([]corev1.EndpointSubset)
	if subsets != nil {
		return (&corev1.Endpoints{Subsets: subsets}).DeepCopy().Subsets
	}
	return []corev1.EndpointSubset{}
}

func (h *targetRecord) updateSubsets(subsets []corev1.EndpointSubset) *targetRecord {
	h.subsets.Store(subsets)
	return h
}

func (h *targetRecord) updateProbes(probeConfs []fluconf.Config) (bool, error) {
	removedProbes, updatedProbes := h.probes, map[probeKey]prober.StatusProber{}
	for host := range hostItems(h.lastSubsets()) {
		hostConf := fluconf.Config{"host": host.ip, "port": strconv.Itoa(int(host.port))}
		for _, conf := range probeConfs {
			probe := prober.LoadSimpleStatusProber(hostConf.CopyWithAll(conf), func(_ int) error {
				h.c.notifyUpdate(h.key)
				return nil
			})
			if name := probe.Name(); name != "" {
				key := probeKey{h.key, host.hostKey, probe.Name()}
				updatedProbes[key] = probe
				delete(removedProbes, key)
				if loaded, _ := h.c.statusUpdater.Start(key, probe); !loaded {
					h.c.logger.Printf("[healthcheck] %s/%s: start %v", key.namespace, key.name, probe)
				}
			}
		}
	}
	for key, probe := range removedProbes {
		h.c.statusUpdater.Stop(key)
		h.c.logger.Printf("[healthcheck] %s/%s: stop %v", key.namespace, key.name, probe)
	}
	h.probeConfs, h.probes = probeConfs, updatedProbes
	return len(probeConfs) > 0, nil
}

func (h *targetRecord) updateSources(sourceConfs []fluconf.Config, ready bool) (bool, error) {
	removedSources, updatedSources := h.sources, map[sourceKey]prober.StatusProber{}
	for _, sourceConf := range sourceConfs {
		source, err := src.Loader(sourceConf, func(_ *src.LoadResult) {
			h.c.notifyUpdate(h.key)
		}, h.c.logger)
		if err != nil {
			return false, err
		}
		key := sourceKey{h.key, source.Name()}
		delete(removedSources, key)
		updatedSources[key] = source
		if loaded, _ := h.c.statusUpdater.Start(key, source); !loaded {
			h.c.logger.Printf("[sources] %s/%s: start %v", key.namespace, key.name, source)
		}
	}
	for key, source := range removedSources {
		h.c.statusUpdater.Stop(key)
		h.c.logger.Printf("[sources] %s/%s: stop %v", key.namespace, key.name, source)
	}
	h.sourceConfs, h.sources = sourceConfs, updatedSources
	return len(sourceConfs) > 0, nil
}

func (c *endpointsImporter) updateTarget(endpoints *corev1.Endpoints, probeConfs []fluconf.Config, sourceConfs []fluconf.Config) error {
	targets, targetKey := c.targets, objectKey{namespace: endpoints.GetNamespace(), name: endpoints.GetName()}
	target, targetOk := targets[targetKey]
	if !targetOk && len(probeConfs) == 0 && len(sourceConfs) == 0 {
		return nil
	}
	if !targetOk {
		target = &targetRecord{c: c, key: targetKey}
		targets[targetKey] = target
	}
	target.updateSubsets(endpoints.Subsets)
	probes, errProbes := target.updateProbes(probeConfs)
	sources, errSources := target.updateSources(sourceConfs, !probes)
	if errSources != nil || errProbes != nil {
		return fmt.Errorf("update: sources=%v, probes=%v", errSources, errProbes)
	}
	if !probes && !sources {
		delete(targets, targetKey)
	}
	c.notifyUpdate(targetKey)
	return nil
}

func (h *targetRecord) hostStatus(ip string) (status bool, ok bool) {
	status, statusOK := false, false
	for key := range h.probes {
		if key.ip == ip {
			probe, probeOK := h.c.statusUpdater.Status(key)
			switch {
			case !probeOK:
				continue
			case probe.(prober.StatusWeight).StatusWeight() < 0:
				return false, true
			case probe.(prober.StatusWeight).StatusWeight() > 0:
				status, statusOK = true, true
			}
		}
	}
	return status, statusOK
}

func (h *targetRecord) buildPatch() ([]byte, bool, error) {
	updateSubsets := []corev1.EndpointSubset{}
	subsets, update := h.subsetsToPatch()
	for _, subset := range subsets {
		updateSubset := corev1.EndpointSubset{Ports: subset.Ports}
		for _, addr := range subset.NotReadyAddresses {
			addrs := &updateSubset.NotReadyAddresses
			if status, statusOK := h.hostStatus(addr.IP); statusOK && status {
				addrs = &updateSubset.Addresses
				update = true
			}
			*addrs = append(*addrs, addr)
		}
		for _, addr := range subset.Addresses {
			addrs := &updateSubset.Addresses
			if status, statusOK := h.hostStatus(addr.IP); statusOK && !status {
				addrs = &updateSubset.NotReadyAddresses
				update = true
			}
			*addrs = append(*addrs, addr)
		}
		updateSubsets = append(updateSubsets, updateSubset)
	}
	if update {
		patch, err := json.Marshal(map[string]interface{}{"subsets": updateSubsets})
		return patch, err == nil, err
	}
	return nil, false, nil
}

func (h *targetRecord) sourceResults() ([]src.LoadResult, bool) {
	results, overwrite := []src.LoadResult{}, false
	for key := range h.sources {
		if source, sourceOK := h.c.statusUpdater.Status(key); sourceOK {
			result := source.(*src.LoadResult)
			if result.Overwrite {
				overwrite = true
			}
			results = append(results, *result)
		}
	}
	return results, overwrite
}

func (h *targetRecord) subsetsToPatch() ([]corev1.EndpointSubset, bool) {
	sources, overwrite := h.sourceResults()
	return buildSubsets(h.lastSubsets(), sources, overwrite, len(h.probeConfs) > 0)
}

type hostItem struct {
	hostKey
	protocol string
}

func hostItems(subsets []corev1.EndpointSubset) map[hostItem]struct{} {
	hosts, ok := map[hostItem]struct{}{}, struct{}{}
	for _, subset := range subsets {
		for _, port := range subset.Ports {
			for _, addr := range subset.NotReadyAddresses {
				if addr.IP != "" && port.Port > 0 {
					hosts[hostItem{hostKey{addr.IP, port.Port}, string(port.Protocol)}] = ok
				}
			}
			for _, addr := range subset.Addresses {
				if addr.IP != "" && port.Port > 0 {
					hosts[hostItem{hostKey{addr.IP, port.Port}, string(port.Protocol)}] = ok
				}
			}
		}
	}
	return hosts
}

func subsetMatches(subset corev1.EndpointSubset, source src.LoadResult) bool {
	if len(source.Ports) != len(subset.Ports) {
		return false
	}
	sourcePorts := map[int]string{}
	for _, port := range source.Ports {
		sourcePorts[port] = source.Protocol
	}
	for _, port := range subset.Ports {
		if protocol, ok := sourcePorts[int(port.Port)]; !ok || (protocol != string(port.Protocol)) {
			return false
		}
	}
	return true
}

func toEndpointPorts(ports []int, protocol string) []corev1.EndpointPort {
	ret := make([]corev1.EndpointPort, len(ports))
	for i, port := range ports {
		ret[i] = corev1.EndpointPort{Port: int32(port), Protocol: corev1.Protocol(protocol)}
	}
	return ret
}

func stringsContains(strings []string, str string) bool {
	for _, s := range strings {
		if s == str {
			return true
		}
	}
	return false
}

func excludeAddresses(subset corev1.EndpointSubset, excludeIPs []string) (corev1.EndpointSubset, bool) {
	addresses, notReadyAddresses, updated := []corev1.EndpointAddress(nil), []corev1.EndpointAddress(nil), false
	for _, addr := range subset.Addresses {
		if !stringsContains(excludeIPs, addr.IP) {
			addresses = append(addresses, addr)
		} else {
			updated = true
		}
	}
	for _, addr := range subset.NotReadyAddresses {
		if !stringsContains(excludeIPs, addr.IP) {
			notReadyAddresses = append(notReadyAddresses, addr)
		} else {
			updated = true
		}
	}
	if !updated {
		return subset, false
	}
	return corev1.EndpointSubset{Ports: subset.Ports, Addresses: addresses, NotReadyAddresses: notReadyAddresses}, true
}

func includeAddresses(subset corev1.EndpointSubset, includeIPs []string, notReady bool) (corev1.EndpointSubset, bool) {
	addresses, updated := &subset.Addresses, false
	if notReady {
		addresses = &subset.NotReadyAddresses
	}
include:
	for _, ip := range includeIPs {
		for _, addr := range subset.Addresses {
			if addr.IP == ip {
				continue include
			}
		}
		for _, addr := range subset.NotReadyAddresses {
			if addr.IP == ip {
				continue include
			}
		}
		*addresses = append(*addresses, corev1.EndpointAddress{IP: ip})
		updated = true
	}
	return subset, updated
}

func buildSubsets(subsets []corev1.EndpointSubset, sources []src.LoadResult, overwrite, notReady bool) ([]corev1.EndpointSubset, bool) {
	if len(sources) == 0 {
		return subsets, false
	}
	newSubsets, sourceMappings, update := make([]corev1.EndpointSubset, 0, len(sources)),
		map[*src.LoadResult]*corev1.EndpointSubset{}, false
source:
	for isource, source := range sources {
		for isubset, subset := range subsets {
			if subsetMatches(subset, source) {
				sourceMappings[&sources[isource]] = &subsets[isubset]
				continue source
			}
		}
		for isubset, subset := range newSubsets {
			if subsetMatches(subset, source) {
				sourceMappings[&sources[isource]] = &newSubsets[isubset]
				continue source
			}
		}
		newSubsets = append(newSubsets, corev1.EndpointSubset{Ports: toEndpointPorts(source.Ports, source.Protocol)})
		sourceMappings[&sources[isource]], update = &newSubsets[len(newSubsets)-1], true
	}

	sourceIPs, ok, excluded, included := map[string]struct{}{}, struct{}{}, false, false
	for isource, source := range sources {
		sourceSubset := sourceMappings[&sources[isource]]
		for isubset := range subsets {
			subset := &subsets[isubset]
			if subset != sourceSubset {
				*subset, excluded = excludeAddresses(*subset, source.IPs)
				update = update || excluded
			}
		}
		for isubset := range newSubsets {
			subset := &newSubsets[isubset]
			if subset != sourceSubset {
				*subset, excluded = excludeAddresses(*subset, source.IPs)
				update = update || excluded
			}
		}
		*sourceSubset, included = includeAddresses(*sourceSubset, source.IPs, notReady)
		update = update || included
		for _, ip := range source.IPs {
			sourceIPs[ip] = ok
		}
	}
	resultSubsets := []corev1.EndpointSubset{}
	for isubset := range subsets {
		subset, delIPs, keep := &subsets[isubset], []string{}, false
		for _, addr := range subset.Addresses {
			if _, sourceOK := sourceIPs[addr.IP]; !sourceOK && overwrite {
				delIPs = append(delIPs, addr.IP)
			} else {
				keep = true
			}
		}
		for _, addr := range subset.NotReadyAddresses {
			if _, sourceOK := sourceIPs[addr.IP]; !sourceOK && overwrite {
				delIPs = append(delIPs, addr.IP)
			} else {
				keep = true
			}
		}
		if len(delIPs) > 0 {
			*subset, _ = excludeAddresses(*subset, delIPs)
		}
		keep = keep && (len(subset.Addresses) > 0 || len(subset.NotReadyAddresses) > 0)
		if keep {
			resultSubsets = append(resultSubsets, *subset)
		}
		update = update || len(delIPs) > 0 || !keep
	}
	for _, subset := range newSubsets {
		resultSubsets = append(resultSubsets, subset)
	}
	return resultSubsets, update
}
