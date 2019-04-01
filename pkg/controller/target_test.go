package controller

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func Test_buildSubsets(t *testing.T) {
	tests := []struct {
		name        string
		subsets     []corev1.EndpointSubset
		sources     []SourceLoadResult
		overwrite   bool
		notReady    bool
		wantSubsets []corev1.EndpointSubset
		wantUpdate  bool
	}{
		{"case-empty", []corev1.EndpointSubset{}, []SourceLoadResult{}, false, false, []corev1.EndpointSubset{}, false},
		{"case-simple-add.0", []corev1.EndpointSubset{}, []SourceLoadResult{
			{IPs: []string{"1.1.1.1"}, Ports: []int{80, 443}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-simple-add.1", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"2.2.2.2"}, Ports: []int{80, 443}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}, {IP: "2.2.2.2"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-nil", nil, []SourceLoadResult{}, false, false, nil, false},
		{"case-nil-add", nil, []SourceLoadResult{
			{IPs: []string{"1.1.1.1"}, Ports: []int{80, 443}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-unchange.0", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, false},
		{"case-unchange.1", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"1.1.1.1"}, Ports: []int{80, 443}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, false},
		{"case-update-port.0", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"1.1.1.1"}, Ports: []int{80}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-update-port.1", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"1.1.1.1"}, Ports: []int{80, 443}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-merge-source.0", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "4.4.4.4"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"2.2.2.2"}, Ports: []int{80, 443}, Protocol: "TCP"},
			{IPs: []string{"3.3.3.3"}, Ports: []int{80, 443}, Protocol: "TCP"},
			{IPs: []string{"5.5.5.5"}, Ports: []int{443}, Protocol: "TCP"},
		}, false, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}, {IP: "2.2.2.2"}, {IP: "3.3.3.3"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "4.4.4.4"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "5.5.5.5"}}, Ports: []corev1.EndpointPort{{Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-overwrite", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "4.4.4.4"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"2.2.2.2"}, Ports: []int{80, 443}, Protocol: "TCP"},
			{IPs: []string{"3.3.3.3"}, Ports: []int{80, 443}, Protocol: "TCP"},
			{IPs: []string{"5.5.5.5"}, Ports: []int{443}, Protocol: "TCP"},
		}, true, false, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "2.2.2.2"}, {IP: "3.3.3.3"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "5.5.5.5"}}, Ports: []corev1.EndpointPort{{Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
		{"case-not-ready", []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "4.4.4.4"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
		}, []SourceLoadResult{
			{IPs: []string{"2.2.2.2"}, Ports: []int{80, 443}, Protocol: "TCP"},
			{IPs: []string{"3.3.3.3"}, Ports: []int{80, 443}, Protocol: "TCP"},
			{IPs: []string{"5.5.5.5"}, Ports: []int{443}, Protocol: "TCP"},
		}, false, true, []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}, NotReadyAddresses: []corev1.EndpointAddress{{IP: "2.2.2.2"}, {IP: "3.3.3.3"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}},
			{Addresses: []corev1.EndpointAddress{{IP: "4.4.4.4"}}, Ports: []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
			{NotReadyAddresses: []corev1.EndpointAddress{{IP: "5.5.5.5"}}, Ports: []corev1.EndpointPort{{Port: 443, Protocol: corev1.ProtocolTCP}}},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSubsets, gotUpdate := buildSubsets(tt.subsets, tt.sources, tt.overwrite, tt.notReady)
			if !reflect.DeepEqual(gotSubsets, tt.wantSubsets) {
				t.Errorf("buildSubsets() gotSubsets = %v, want %v", gotSubsets, tt.wantSubsets)
			}
			if gotUpdate != tt.wantUpdate {
				t.Errorf("buildSubsets() gotUpdate = %v, want %v", gotUpdate, tt.wantUpdate)
			}
		})
	}
}
